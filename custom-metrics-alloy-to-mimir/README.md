From project 
Deploy mimir and grafana with docker

`docker compose -up -d`

Test the URLs are working
```
curl http://localhost:3000/api/health
curl http://localhost:9009/ready
```

Go to http://localhost:3000 confirm you have the mimir (prometheus) datasource

run our mock server that produces some prometheus metrics:

`go run prom-metrics-demo-server/demo-server.go`

from another terminal in project dir:

Run alloy to scrape and push metrics (you'll need to have grafana alloy binary installed - this would mimic what we'd do on cloud)

`alloy run alloy-config.river`

