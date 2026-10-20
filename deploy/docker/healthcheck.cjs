const http = require('node:http');
const request = http.get('http://127.0.0.1:8092/healthz', (response) => {
  response.resume();
  process.exitCode = response.statusCode === 204 ? 0 : 1;
});
request.setTimeout(3000, () => request.destroy());
request.on('error', () => { process.exitCode = 1; });
