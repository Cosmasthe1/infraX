const http = require('http');
const PORT = process.env.PORT || 8080;

const server = http.createServer((req, res) => {
  if (req.url === '/health') {
    res.writeHead(200, {'Content-Type': 'text/plain'});
    res.end('ok');
    return;
  }
  res.writeHead(200, {'Content-Type': 'text/plain'});
  res.end('hello from sample-app');
});

server.listen(PORT, () => {
  console.log(`sample-app listening on ${PORT}`);
});
