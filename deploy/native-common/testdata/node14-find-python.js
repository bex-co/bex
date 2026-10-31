// Exact node-gyp5.1.1 findPython method, retained for interpreter-order controls.
// Source: https://github.com/nodejs/node/blob/v14.21.3/deps/npm/node_modules/node-gyp/lib/find-python.js
// Copyright (c) 2012 Nathan Rajlich <nathan@tootallnate.net>, MIT license.
module.exports = {
  findPython: function findPython () {
    const SKIP = 0; const FAIL = 1
    var toCheck = getChecks.apply(this)

    function getChecks () {
      if (this.env.NODE_GYP_FORCE_PYTHON) {
        return [{
          before: () => {
            this.addLog(
              'checking Python explicitly set from NODE_GYP_FORCE_PYTHON')
            this.addLog('- process.env.NODE_GYP_FORCE_PYTHON is ' +
              `"${this.env.NODE_GYP_FORCE_PYTHON}"`)
          },
          check: this.checkCommand,
          arg: this.env.NODE_GYP_FORCE_PYTHON
        }]
      }

      var checks = [
        {
          before: () => {
            if (!this.configPython) {
              this.addLog(
                'Python is not set from command line or npm configuration')
              return SKIP
            }
            this.addLog('checking Python explicitly set from command line or ' +
              'npm configuration')
            this.addLog('- "--python=" or "npm config get python" is ' +
              `"${this.configPython}"`)
          },
          check: this.checkCommand,
          arg: this.configPython
        },
        {
          before: () => {
            if (!this.env.PYTHON) {
              this.addLog('Python is not set from environment variable ' +
                'PYTHON')
              return SKIP
            }
            this.addLog('checking Python explicitly set from environment ' +
              'variable PYTHON')
            this.addLog(`- process.env.PYTHON is "${this.env.PYTHON}"`)
          },
          check: this.checkCommand,
          arg: this.env.PYTHON
        },
        {
          before: () => { this.addLog('checking if "python" can be used') },
          check: this.checkCommand,
          arg: 'python'
        },
        {
          before: () => { this.addLog('checking if "python2" can be used') },
          check: this.checkCommand,
          arg: 'python2'
        },
        {
          before: () => { this.addLog('checking if "python3" can be used') },
          check: this.checkCommand,
          arg: 'python3'
        }
      ]

      if (this.win) {
        checks.push({
          before: () => {
            this.addLog(
              'checking if the py launcher can be used to find Python 2')
          },
          check: this.checkPyLauncher
        })
        for (var i = 0; i < this.winDefaultLocations.length; ++i) {
          const location = this.winDefaultLocations[i]
          checks.push({
            before: () => {
              this.addLog('checking if Python is ' +
                `${location}`)
            },
            check: this.checkExecPath,
            arg: location
          })
        }
      }

      return checks
    }

    function runChecks (err) {
      this.log.silly('runChecks: err = %j', (err && err.stack) || err)

      const check = toCheck.shift()
      if (!check) {
        return this.fail()
      }

      const before = check.before.apply(this)
      if (before === SKIP) {
        return runChecks.apply(this)
      }
      if (before === FAIL) {
        return this.fail()
      }

      const args = [runChecks.bind(this)]
      if (check.arg) {
        args.unshift(check.arg)
      }
      check.check.apply(this, args)
    }

    runChecks.apply(this)
  }
}
