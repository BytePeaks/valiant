## In order to test locally (but on k8s alike cluster):

### Ubuntu:

1. Install `kubectl`
```bash
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo mv kubectl /usr/bin/kubectl
sudo chmod a+x  /usr/bin/kubectl
# Check
kubectl version --client
```

2. Install cluster (k3d)
```bash
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
# Check
k3d version
```

3. Install helm (for prometheus deployment)
```bash
curl -L https://get.helm.sh/helm-v3.14.2-linux-amd64.tar.gz -o helm.tar.gz
tar -xzf helm.tar.gz
sudo mv linux-amd64/helm /usr/local/bin/helm
sudo chmod +x /usr/local/bin/helm
rm -rf linux-amd64 helm.tar.gz
# Check
helm version
```

4. Create cluster k3d
```bash
k3d cluster create valiant --servers 1 --agents 1 --k3s-arg "--disable=traefik@server:0"
kubectl get nodes
```


5. Add Prometheus helm
```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
kubectl create namespace monitoring
helm install kps prometheus-community/kube-prometheus-stack -n monitoring
```

6. Build images locally and push to k3d
```bash
cd backend/
docker build -t valiant-backend:latest .

# then

cd ../frontend
docker build -t valiant-frontend:latest .

# after build just push to cluster like

k3d image import valiant-backend:latest -c valiant
k3d image import valiant-frontend:latest -c valiant
```

7. Install system on cluster
```bash
cd deploy/kubernetes
kubectl apply -k .
# wait a bit backend may restart couple of times
kubectl get pods -n valiant
# 3 pods all in Running state means deploy complete and valid!
```