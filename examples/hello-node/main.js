// hello-node: answers 200 OK on every GET. MESSAGE and PORT are platform-injected;
// MESSAGE is overridable via bex.yml envVars so a redeploy changes the response visibly.
const http = require('http')

const message = process.env.MESSAGE || 'hello from bex'
const port = parseInt(process.env.PORT || '3000', 10)

http
  .createServer((req, res) => {
    res.writeHead(200, { 'Content-Type': 'text/plain' })
    res.end(message + '\n')
  })
  .listen(port, () => {
    console.log(`hello-node listening on ${port}: "${message}"`)
  })
