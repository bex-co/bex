'use strict'

exports = module.exports = lifecycle
exports.makeEnv = makeEnv

const spawn = require('./lib/spawn')
const { execute } = require('@yarnpkg/shell')
const { npath } = require('@yarnpkg/fslib')
const path = require('path')
const Stream = require('stream').Stream
const fs = require('fs')
const chain = require('slide').chain
const uidNumber = require('uid-number')
const umask = require('umask')
const byline = require('@pnpm/byline')
const { PnpmError } = require('@pnpm/error')
const resolveFrom = require('resolve-from')
const { PassThrough } = require('stream')
const extendPath = require('./lib/extendPath')

let DEFAULT_NODE_GYP_PATH
try {
  DEFAULT_NODE_GYP_PATH = resolveFrom(__dirname, 'node-gyp/bin/node-gyp')
} catch (err) {}

const hookStatCache = new Map()

let PATH = 'PATH'

// windows calls it's path 'Path' usually, but this is not guaranteed.
if (process.platform === 'win32') {
  PATH = 'Path'
  Object.keys(process.env).forEach(e => {
    if (e.match(/^PATH$/i)) {
      PATH = e
    }
  })
}

function logid (pkg, stage) {
  return `${pkg._id}~${stage}:`
}

function hookStat (dir, stage, cb) {
  const hook = path.join(dir, '.hooks', stage)
  const cachedStatError = hookStatCache.get(hook)

  if (cachedStatError === undefined) {
    return fs.stat(hook, statError => {
      hookStatCache.set(hook, statError)
      cb(statError)
    })
  }

  return setImmediate(() => cb(cachedStatError))
}

function lifecycle (pkg, stage, wd, opts) {
  return new Promise((resolve, reject) => {
    while (pkg && pkg._data) pkg = pkg._data
    if (!pkg) return reject(new Error('Invalid package data'))

    opts.log.info('lifecycle', logid(pkg, stage), pkg._id)
    if (!pkg.scripts) pkg.scripts = {}

    if (stage === 'prepublish' && opts.ignorePrepublish) {
      opts.log.info('lifecycle', logid(pkg, stage), 'ignored because ignore-prepublish is set to true', pkg._id)
      delete pkg.scripts.prepublish
    }

    hookStat(opts.dir, stage, statError => {
      // makeEnv is a slow operation. This guard clause prevents makeEnv being called
      // and avoids a ton of unnecessary work, and results in a major perf boost.
      if (!pkg.scripts[stage] && statError) return resolve()

      validWd(wd || path.resolve(opts.dir, pkg.name), (er, wd) => {
        if (er) return reject(er)

        // set the env variables, then run scripts as a child process.
        const env = makeEnv(pkg, opts)
        env.npm_lifecycle_event = stage
        env.npm_node_execpath = env.NODE = env.NODE || process.execPath
        env.npm_package_json = path.join(wd, 'package.json')
        if (process.pkg != null) {
          // If the pnpm CLI was bundled by vercel/pkg then we cannot use the js path for npm_execpath
          // because in that case the js is in a virtual filesystem inside the executor.
          // Instead, we use the path to the exe file.
          env.npm_execpath = process.execPath
        } else {
          env.npm_execpath = require.main ? require.main.filename : process.cwd()
        }
        env.INIT_CWD = process.cwd()
        env.npm_config_node_gyp = env.npm_config_node_gyp || DEFAULT_NODE_GYP_PATH
        if (opts.extraEnv) {
          for (const [key, value] of Object.entries(opts.extraEnv)) {
            env[key] = value
          }
        }

        // 'nobody' typically doesn't have permission to write to /tmp
        // even if it's never used, sh freaks out.
        if (!opts.unsafePerm) {
          const tmpdir = path.join(wd, 'node_modules', '.tmp')
          try {
            fs.mkdirSync(tmpdir, { recursive: true })
          } catch (err) {
            if (err.code !== 'EEXIST') throw err
          }
          env.TMPDIR = tmpdir
        }

        lifecycle_(pkg, stage, wd, opts, env, (er) => {
          if (er) return reject(er)
          return resolve()
        })
      })
    })
  })
}

function lifecycle_ (pkg, stage, wd, opts, env, cb) {
  env[PATH] = extendPath(wd, env[PATH], path.join(__dirname, 'node-gyp-bin'), opts)

  let packageLifecycle = pkg.scripts && Object.prototype.hasOwnProperty.call(pkg.scripts, stage)

  if (opts.ignoreScripts) {
    opts.log.info('lifecycle', logid(pkg, stage), 'ignored because ignore-scripts is set to true', pkg._id)
    packageLifecycle = false
  } else if (packageLifecycle) {
    // define this here so it's available to all scripts.
    env.npm_lifecycle_script = pkg.scripts[stage]
  } else {
    opts.log.silly('lifecycle', logid(pkg, stage), `no script for ${stage}, continuing`)
  }

  function done (er) {
    if (er) {
      if (opts.force) {
        opts.log.info('lifecycle', logid(pkg, stage), 'forced, continuing', er)
        er = null
      } else if (opts.failOk) {
        opts.log.warn('lifecycle', logid(pkg, stage), 'continuing anyway', er.message)
        er = null
      }
    }
    cb(er)
  }

  chain(
    [
      packageLifecycle && [runPackageLifecycle, pkg, stage, env, wd, opts],
      [runHookLifecycle, pkg, stage, env, wd, opts]
    ],
    done
  )
}

function validWd (d, cb) {
  fs.stat(d, (er, st) => {
    if (er || !st.isDirectory()) {
      const p = path.dirname(d)
      if (p === d) {
        return cb(new Error('Could not find suitable wd'))
      }
      return validWd(p, cb)
    }
    return cb(null, d)
  })
}

function runPackageLifecycle (pkg, stage, env, wd, opts, cb) {
  // run package lifecycle scripts in the package root, or the nearest parent.
  const cmd = env.npm_lifecycle_script

  const note = `\n> ${pkg._id} ${stage} ${wd}\n> ${cmd}\n`
  runCmd(note, cmd, pkg, env, stage, wd, opts, cb)
}

let running = false
const queue = []
function dequeue () {
  running = false
  if (queue.length) {
    const r = queue.shift()
    runCmd.apply(null, r)
  }
}

function runCmd (note, cmd, pkg, env, stage, wd, opts, cb) {
  if (opts.runConcurrently !== true) {
    if (running) {
      queue.push([note, cmd, pkg, env, stage, wd, opts, cb])
      return
    }

    running = true
  }
  opts.log.pause()
  let unsafe = opts.unsafePerm
  const user = unsafe ? null : opts.user
  const group = unsafe ? null : opts.group

  if (opts.log.level !== 'silent') {
    opts.log.clearProgress()
    console.log(note)
    opts.log.showProgress()
  }
  opts.log.verbose('lifecycle', logid(pkg, stage), 'unsafe-perm in lifecycle', unsafe)

  if (process.platform === 'win32') {
    unsafe = true
  }

  if (unsafe) {
    runCmd_(cmd, pkg, env, wd, opts, stage, unsafe, 0, 0, cb)
  } else {
    uidNumber(user, group, (er, uid, gid) => {
      runCmd_(cmd, pkg, env, wd, opts, stage, unsafe, uid, gid, cb)
    })
  }
}

function runCmd_ (cmd, pkg, env, wd, opts, stage, unsafe, uid, gid, cb_) {
  function cb (er) {
    cb_.apply(null, arguments)
    opts.log.resume()
    process.nextTick(dequeue)
  }

  const conf = {
    cwd: wd,
    env,
    stdio: opts.stdio || [0, 1, 2]
  }

  if (!unsafe) {
    conf.uid = uid ^ 0
    conf.gid = gid ^ 0
  }

  let sh = 'sh'
  let shFlag = '-c'

  const customShell = opts.scriptShell

  if (customShell) {
    sh = customShell
  } else if (process.platform === 'win32') {
    sh = process.env.comspec || 'cmd'
    shFlag = '/d /s /c'
    conf.windowsVerbatimArguments = true
  }

  opts.log.verbose('lifecycle', logid(pkg, stage), 'PATH:', env[PATH])
  opts.log.verbose('lifecycle', logid(pkg, stage), 'CWD:', wd)
  opts.log.silly('lifecycle', logid(pkg, stage), 'Args:', [shFlag, cmd])

  if (opts.shellEmulator) {
    const execOpts = { cwd: npath.toPortablePath(wd), env }
    if (opts.stdio === 'pipe') {
      const stdout = new PassThrough()
      const stderr = new PassThrough()
      byline(stdout).on('data', data => {
        opts.log.verbose('lifecycle', logid(pkg, stage), 'stdout', data.toString())
      })
      byline(stderr).on('data', data => {
        opts.log.verbose('lifecycle', logid(pkg, stage), 'stderr', data.toString())
      })
      execOpts.stdout = stdout
      execOpts.stderr = stderr
    }
    execute(cmd, [], execOpts)
      .then((code) => {
        opts.log.silly('lifecycle', logid(pkg, stage), 'Returned: code:', code)
        let er
        if (code) {
          er = new Error(`Exit status ${code}`)
          er.errno = code
        }
        procError(er)
      })
      .catch((err) => procError(err))
    return
  }

  const proc = spawn(sh, [shFlag, cmd], conf, opts.log)

  proc.on('error', procError)
  proc.on('close', (code, signal) => {
    opts.log.silly('lifecycle', logid(pkg, stage), 'Returned: code:', code, ' signal:', signal)
    let err
    if (signal) {
      err = new PnpmError('CHILD_PROCESS_FAILED', `Command failed with signal "${signal}"`)
      process.kill(process.pid, signal)
    } else if (code) {
      err = new PnpmError('CHILD_PROCESS_FAILED', `Exit status ${code}`)
      err.errno = code
    }
    procError(err)
  })
  byline(proc.stdout).on('data', data => {
    opts.log.verbose('lifecycle', logid(pkg, stage), 'stdout', data.toString())
  })
  byline(proc.stderr).on('data', data => {
    opts.log.verbose('lifecycle', logid(pkg, stage), 'stderr', data.toString())
  })
  process.once('SIGTERM', procKill)
  process.once('SIGINT', procInterrupt)
  process.on('exit', procKill)

  function procError (er) {
    if (er) {
      opts.log.info('lifecycle', logid(pkg, stage), `Failed to exec ${stage} script`)
      er.message = `${pkg._id} ${stage}: \`${cmd}\`\n${er.message}`
      if (er.code !== 'EPERM') {
        er.code = 'ELIFECYCLE'
      }
      fs.stat(opts.dir, (statError) => {
        if (statError && statError.code === 'ENOENT' && opts.dir.split(path.sep).slice(-1)[0] === 'node_modules') {
          opts.log.warn('', 'Local package.json exists, but node_modules missing, did you mean to install?')
        }
      })
      er.pkgid = pkg._id
      er.stage = stage
      er.script = cmd
      er.pkgname = pkg.name
    }
    process.removeListener('SIGTERM', procKill)
    process.removeListener('SIGINT', procKill)
    process.removeListener('SIGINT', procInterrupt)
    process.removeListener('exit', procKill)
    return cb(er)
  }
  let called = false
  function procKill () {
    if (called) return
    called = true
    proc.kill()
  }
  function procInterrupt () {
    proc.kill('SIGINT')
    process.once('SIGINT', procKill)
  }
}

function runHookLifecycle (pkg, stage, env, wd, opts, cb) {
  hookStat(opts.dir, stage, er => {
    if (er) return cb()
    const cmd = path.join(opts.dir, '.hooks', stage)
    const note = `\n> ${pkg._id} ${stage} ${wd}\n> ${cmd}`
    runCmd(note, cmd, pkg, env, stage, wd, opts, cb)
  })
}

function makeEnv (data, opts, prefix, env) {
  prefix = prefix || 'npm_package_'
  if (!env) {
    env = {}
    for (const i in process.env) {
      if (!i.match(/^npm_/) && (!i.match(/^PATH$/i) || i === PATH)) {
        env[i] = process.env[i]
      }
    }

    // express and others respect the NODE_ENV value.
    if (opts.production) env.NODE_ENV = 'production'
  } else if (!Object.prototype.hasOwnProperty.call(data, '_lifecycleEnv')) {
    Object.defineProperty(data, '_lifecycleEnv',
      {
        value: env,
        enumerable: false
      }
    )
  }

  if (opts.nodeOptions) env.NODE_OPTIONS = opts.nodeOptions

  for (const i in data) {
    if (i.charAt(0) !== '_') {
      const envKey = (prefix + i).replace(/[^a-zA-Z0-9_]/g, '_')
      if (
        !['name', 'version', 'config', 'engines', 'bin'].includes(i) &&
        !prefix.startsWith('npm_package_config_') &&
        !prefix.startsWith('npm_package_engines_') &&
        !prefix.startsWith('npm_package_bin_')
      ) {
        continue
      }
      if (data[i] && typeof data[i] === 'object') {
        try {
          // quick and dirty detection for cyclical structures
          JSON.stringify(data[i])
          makeEnv(data[i], opts, `${envKey}_`, env)
        } catch (ex) {
          // usually these are package objects.
          // just get the path and basic details.
          const d = data[i]
          makeEnv(
            { name: d.name, version: d.version, path: d.path },
            opts,
            `${envKey}_`,
            env
          )
        }
      } else {
        env[envKey] = String(data[i])
        env[envKey] = env[envKey].includes('\n')
          ? JSON.stringify(env[envKey])
          : env[envKey]
      }
    }
  }

  if (prefix !== 'npm_package_') return env

  prefix = 'npm_config_'
  const pkgConfig = {}
  const pkgVerConfig = {}
  const namePref = `${data.name}:`
  const verPref = `${data.name}@${data.version}:`

  Object.keys(opts.config).forEach(i => {
    // in some rare cases (e.g. working with nerf darts), there are segmented
    // "private" (underscore-prefixed) config names -- don't export
    if ((i.charAt(0) === '_' && i.indexOf(`_${namePref}`) !== 0) || i.match(/:_/)) {
      return
    }
    let value = opts.config[i]
    if (value instanceof Stream || Array.isArray(value)) return
    if (i.match(/umask/)) value = umask.toString(value)
    if (!value) value = ''
    else if (typeof value === 'number') value = `${value}`
    else if (typeof value !== 'string') value = JSON.stringify(value)

    value = value.includes('\n')
      ? JSON.stringify(value)
      : value
    i = i.replace(/^_+/, '')
    let k
    if (i.indexOf(namePref) === 0) {
      k = i.substring(namePref.length).replace(/[^a-zA-Z0-9_]/g, '_')
      pkgConfig[k] = value
    } else if (i.indexOf(verPref) === 0) {
      k = i.substring(verPref.length).replace(/[^a-zA-Z0-9_]/g, '_')
      pkgVerConfig[k] = value
    }
    const envKey = (prefix + i).replace(/[^a-zA-Z0-9_]/g, '_')
    env[envKey] = value
  })

  prefix = 'npm_package_config_'
  ;[pkgConfig, pkgVerConfig].forEach(conf => {
    for (const i in conf) {
      const envKey = (prefix + i)
      env[envKey] = conf[i]
    }
  })

  return env
}
