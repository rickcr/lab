This approach each metrics Read produces a String
in Prometheus Exposition Format.
You save the read of the batch of metrics (from call to /metrics
ultimately to a file with a .prom extension.

You then rely on alloy's prometheus.exporter.textfile "metrics_reader" 
to to scrape that file into a metric that it would load into Mimir DB

For these metrics we could send them to a Pulsar queue.

And downstream we'd read them off with Alloy.


