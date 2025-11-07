# Intro
Purpose of this project is to demonstrate using a Grafana Alloy monitoring stack of Kubernetes. It uses the following components:

1. Grafana Alloy - Scraping metrics and logs
2. Loki - Log Capture/Aggregation 
3. Mimir - Metrics storage
4. MinIO - Persistent storage (for Loki and Mimir)
5. In this project we also deploy an open source web app "Mealie" so that we can mimic capture application logs.

This project started by following a lot of what was done here: 
https://grafana.com/docs/alloy/latest/monitor/monitor-kubernetes-logs/


# Start Docker and create cluster with Kind
If you already are comfortable setting up a kubernetes cluster other ways (eg rancher desktop, etc.), you can skip this step. 
For simplicity just using kind to set up a cluster.

Navigate to where the manifest files are eg {path}/grafana-alloy-demo/ 
 
start docker (or restart)

`sudo systemctl start docker`

Use kind and create a cluster using the kind.yml which just makes two workers

`kind create cluster --config kind.yaml`

NOTE: So dns issues don't arise, make sure you don't have a custom docker daemon in /etc/docker 
Also, we'll be using the namespace "demo" (The mealie app isntall in the next section will create it though.)

# Deploy our mealie app 

 mealie was just an open source web app that I had used in a tutorial I was going through. We'll use it as well just to have an application deployed in a pod. 

Deploy the service and app that'll use port 9000

`kubectl apply -f mealie-deployment.yaml`

Port forward 9000

`kubectl port-forward svc/mealie 9000:9000 -n demo`

Test Mealie is up: http://localhost:9000/

When the app comes up just hit login a few times, just hitting enter. (We'll be monitoring unauthorized attempts)

# Add grafana helm repo
`helm repo add grafana https://grafana.github.io/helm-charts`

# Deploy minio 
This will be to mimic an s3 storage buckets to store metrics that mimir can use and storage for loki as well.

`helm install minio minio-official/minio -f minio-values.yaml -n demo`

# Deploy loki
`helm install --values loki-values.yaml loki grafana/loki -n demo`

# Deploy grafana
First create a config map of our dashboards we'll use from our json dashboards in the dashboard dir

```
kubectl create configmap grafana-dashboards \
  --from-file=dashboards/ \
  -n demo --save-config
```

Note, if updating the config map you can use the following, which will also work if it hasn't been created yet:

```kubectl create configmap grafana-dashboards \
  --from-file=dashboards/ \
  -n demo \
  --dry-run=client -o yaml | kubectl apply -f -
  ```

Now deploy grafana:

`helm install **n**n**nk**--values grafana-values.yaml grafana grafana/grafana -n demo`

This Helm chart installs Grafana and sets the datasources.datasources.yaml field to the Loki data source configuration.

# Deploy mimir
`helm install mimir grafana/mimir-distributed -n demo -f mimir-values.yaml`

Mimir is a long-term metrics storage system - it's like a database specifically designed for storing Prometheus-style time-series metrics (numbers over time like CPU usage, memory, request counts, etc.). It's the "database" running on the minio "drive for storage"

# Deploy alloy 
	`helm install --values k8s-monitoring-values.yaml k8s grafana/k8s-monitoring -n demo` 

Note, I had to use  integrations to get mimir metrics scanned

# Set up port forwarding for Grafana
Note, I'm forwarding based on service  vs pod, since it should work with restarts. (Pod based is what the docs show, so showing that as well) 

`kubectl port-forward -n demo svc/grafana 3000:80`

(If you don't want to block on that terminal append with &, same thing with future port forwards)

If you want to forward the pod (which is what docs show, but service above should be better)

`export POD_NAME=$(kubectl get pods --namespace demo -l "app.kubernetes.io/name=grafana,app.kubernetes.io/instance=grafana" -o jsonpath="{.items[0].metadata.name}")`
`kubectl --namespace demo port-forward $POD_NAME 3000`

You can now get to Grafana at:   http://localhost:3000/

# Port-forward the Alloy Pod to your local machine:
Similar to above, I port forwarded based on service name, which should be more robust that pod forward shown next.

`kubectl port-forward -n demo svc/k8s-alloy-logs 12345:12345`

If you want to forward the pod (which is what docs show)

`export POD_NAME=$(kubectl get pods --namespace demo  -l "app.kubernetes.io/name=alloy-logs" -o jsonpath="{.items[0].metadata.name}")`
`kubectl --namespace demo port-forward $POD_NAME 12345`  

# Visit pages
 Grafana UI: http://localhost:3000/
 password: adminadminadmin

 Alloy: http://localhost:12345/
 Show's what Alloy is collecting 

# Add Mimir as data source in Grafana
I added this datasource to the helm config, but it was initally created within Grafana as:

Go to Connections → Data sources → Add data source
Select Prometheus
Name: mimir
URL:  http://mimir-nginx.demo.svc.cluster.local/prometheus
Add http header
X-Scope-OrgID with value "demo"

# Dashboards
Dashboards are in the dashboards dir, but they're installed as part of our Grafana deploy,  when we made our config map for the dashboards, and the config map is defined in the grafana helm config:

```
dashboardsConfigMaps:
  default: "grafana-dashboards"
```

# Other testing
In Grafana dashboard explorer you can query loki for logs that are scraped. Eg for mealie logs
{namespace="demo"} |= "mealie"


Grafana logs might show a lot of messages "failed to create fsnotify watcher: too many open files"
If that is the case, I did:
sudo sysctl fs.inotify.max_user_watches=524288
sudo sysctl fs.inotify.max_user_instances=512
and then restarted the grafana pod (just delete it and let it restart)

