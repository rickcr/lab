# Into
This project is demonstrating a case where you woudln't be able to stream metrics 
directly from a REST end point (/metrics) and instead, for this prototype, it writes 
metrics (from a string) to a .prom file that Alloy can scan for and then push to Mimir.

From project 
Deploy mimir and grafana with docker

`docker compose -up -d`

Test the URLs are working
```
curl http://localhost:3000/api/health
curl http://localhost:9009/ready
```
(takes some time about 15 sec or so for mimir on 9009 to be up)

Go to http://localhost:3000 confirm you have the mimir (prometheus) datasource

run our mock server that produces some prometheus metrics:

`go run prom-metrics-demo-server/demo-server.go`

from another terminal in project dir:

Run alloy which, in this project, will be scanning the temp dir in the project for a .prom
file to push to mimir

`alloy run alloy-config.river`

