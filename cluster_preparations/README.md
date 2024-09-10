"Each new cluster must have a service account set up to enable GitHub Actions to deploy the decentralized-feeder to the cluster.
In this folder you find all the neccessary kubernetes manifest files
Create the in this order in the dia-lumina namespace of the respective cluster:
1. kubectl apply -f github-actions-sa.yml
2. kubectl apply -f github-actions-secret.yml 
3. kubectl apply -f role.yml 
4. kubectl apply -f rolebinding-github-actions.yml 

after this is done, then we have to have the secrets created for the respective conduit-node in the dia-lumina namespace. 
13 and 14 in Hetzner.
15 and 16 in Civo.
also the ibm container registry secret. 

we also have to add the github secrets for
K8S_SERVICE_ACCOUNT_TOKEN_CIVO --- token value created during kubectl apply -f github-actions-sa.yml
K8s_CLUSTER_NAME_CIVO 
K8s_CONTEXT_CIVO
K8S_API_SERVER_CIVO
