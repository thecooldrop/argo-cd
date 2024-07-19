kind create cluster && \
kubectl create namespace argocd && \
kubectl apply -f manifests/install.yaml -n argocd && \
kubectl config set-context --current --namespace=argocd;
sleep 60;

kubectl port-forward svc/argocd-server -n argocd 8080:80 > /dev/null 2>&1 &
pid=$!
trap '{
    # echo killing $pid
    kill $pid
}' EXIT


sleep 10;
ADMIN_PASSWORD=$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d; echo)
argocd login localhost:8080 --username admin --password $ADMIN_PASSWORD --insecure

argocd app create --app-namespace argocd \
--dest-namespace argocd \
--dest-server https://kubernetes.default.svc \
--label app=my-app \
--name exampleapp \
--repo https://github.com/thecooldrop/argo-cd \
--revision fix-duplicate-nonnamespaced-resources \
--path duplicate-nonnamespaced-resources \
--insecure

argocd app get exampleapp