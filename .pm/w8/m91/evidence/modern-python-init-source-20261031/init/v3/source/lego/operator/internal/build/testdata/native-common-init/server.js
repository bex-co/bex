const fs = require("node:fs");
const http = require("node:http");
const { expected, runtime } = require("./identity");

const identity = runtime();
const events = [];
let holds = 0;
let completed = 0;
let signals = 0;
function event(value) {
  events.push(value);
  console.log(JSON.stringify({ run: expected.run, event: value }));
}
const server = http.createServer((request, response) => {
  if (request.url === "/ready" || request.url === "/state") {
    response.setHeader("Content-Type", "application/json");
    return response.end(JSON.stringify({ run: expected.run, identity, events }));
  }
  if (request.url !== "/hold" || request.method !== "GET") {
    response.writeHead(404);
    return response.end();
  }
  if (holds++ !== 0) {
    response.writeHead(409);
    return response.end("one held request only");
  }
  event("hold_begin");
  setTimeout(() => {
    response.end("held 雪 λ🙂");
    completed++;
    event("hold_end");
  }, 4000);
});
process.on("SIGTERM", () => {
  signals++;
  event("SIGTERM");
  server.close(() => {
    event("closed");
    const proof = {
      phase: "runtime",
      run: expected.run,
      identity,
      events,
      holds,
      completed,
      signals,
      pass: holds === 1 && completed === 1 && signals === 1,
    };
    fs.writeFileSync("qa-output/runtime.json", JSON.stringify(proof, null, 2) + "\n");
    console.log(JSON.stringify(proof, null, 2));
    if (!proof.pass) process.exitCode = 1;
  });
});
server.listen(Number(process.env.PORT), "0.0.0.0", () => event("listening"));
