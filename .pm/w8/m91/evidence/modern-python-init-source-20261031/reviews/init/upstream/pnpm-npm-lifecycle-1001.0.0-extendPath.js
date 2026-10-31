const fs = require('fs')
const path = require('path')
const which = require('which')

module.exports = (wd, originalPath, nodeGyp, opts) => {
  const pathArr = [...opts.extraBinPaths || []]
  const p = wd.split(/[\\/]node_modules[\\/]/)
  let acc = path.resolve(p.shift())

  // we also unshift the bundled node-gyp-bin folder so that
  // the bundled one will be used for installing things.
  pathArr.unshift(nodeGyp)

  p.forEach(pp => {
    pathArr.unshift(path.join(acc, 'node_modules', '.bin'))
    acc = path.join(acc, 'node_modules', pp)
  })
  pathArr.unshift(path.join(acc, 'node_modules', '.bin'))

  if (shouldPrependCurrentNodeDirToPATH(opts)) {
    // prefer current node interpreter in child scripts
    pathArr.push(path.dirname(process.execPath))
  }

  if (originalPath) pathArr.push(originalPath)
  return pathArr.join(process.platform === 'win32' ? ';' : ':')
}

function shouldPrependCurrentNodeDirToPATH (opts) {
  const cfgsetting = opts.scriptsPrependNodePath
  if (cfgsetting === false || cfgsetting == null) return false
  if (cfgsetting === true) return true

  let isDifferentNodeInPath

  const isWindows = process.platform === 'win32'
  let foundExecPath
  try {
    foundExecPath = which.sync(path.basename(process.execPath), { pathExt: isWindows ? ';' : ':' })
    // Apply `fs.realpath()` here to avoid false positives when `node` is a symlinked executable.
    isDifferentNodeInPath = fs.realpathSync(process.execPath).toUpperCase() !==
        fs.realpathSync(foundExecPath).toUpperCase()
  } catch (e) {
    isDifferentNodeInPath = true
  }

  if (cfgsetting === 'warn-only') {
    if (isDifferentNodeInPath && !shouldPrependCurrentNodeDirToPATH.hasWarned) {
      if (foundExecPath) {
        opts.log.warn('lifecycle', `The node binary used for scripts is ${foundExecPath} but pnpm is using ${process.execPath} itself. Use the \`--scripts-prepend-node-path\` option to include the path for the node binary pnpm was executed with.`)
      } else {
        opts.log.warn('lifecycle', `pnpm is using ${process.execPath} but there is no node binary in the current PATH. Use the \`--scripts-prepend-node-path\` option to include the path for the node binary pnpm was executed with.`)
      }
      shouldPrependCurrentNodeDirToPATH.hasWarned = true
    }

    return false
  }

  return isDifferentNodeInPath
}
