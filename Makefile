k3d-create:
	$(MAKE) k3d-delete
	$(MAKE) k3d-registry-create
	$(MAKE) agent-image-push
	$(MAKE) producer-image-push
	k3d cluster create app --config k3d.yaml --registry-config=registries.yaml --registry-use registry.localhost:5000
	$(MAKE) kube-registry-secret
	$(MAKE) kube-envvar-secret
	$(MAKE) agent-deployment-create

k3d-delete:
	k3d cluster delete app || true
	k3d registry delete registry.localhost || true

k3d-registry-create:
	k3d registry create registry.localhost -p 5000

producer-build:
	cd producer ; GOBIN=/usr/local/bin go install "github.com/ingyamilmolinar/doctorgpt/producer" ; cd -

producer-image-build:
	cd producer ; docker build --progress=plain -f Dockerfile -t producer:latest ../. && cd -

producer-image-push:
	$(MAKE) producer-image-build
	docker tag producer:latest localhost:5000/chatgpt:producer && docker push localhost:5000/chatgpt:producer

producer-deploy:
	$(MAKE) producer-image-push
	$(MAKE) agent-deployment-refresh

producer-logs:
	kubectl logs deployment/agent producer -f

agent-build:
	cd agent ; GOBIN=/usr/local/bin go install "github.com/ingyamilmolinar/doctorgpt/agent" ; cd -

agent-unit-tests:
	cd agent ; go test ./... && cd -

agent-run:
	$(MAKE) agent-install
	cd agent ; /usr/local/bin/agent --logfile testlogs/Linux_2k.log --outdir /tmp/agent-errors --configfile config.yaml; echo $$?; cd -

agent-image-build:
	cd agent ; docker build --progress=plain -f Dockerfile -t agent:latest . && cd -

agent-image-push:
	$(MAKE) agent-image-build
	docker tag agent:latest localhost:5000/chatgpt:agent && docker push localhost:5000/chatgpt:agent

agent-image-push-remote:
	$(MAKE) agent-image-build
	docker tag agent:latest ingyamilmolinar/chatgpt:agent && docker push ingyamilmolinar/chatgpt:agent

agent-deployment-delete:
	kubectl delete deployment agent || true

agent-deployment-create:
	kubectl apply -f agent/deployment.yaml
	kubectl rollout status deployment agent --timeout=60s

agent-deploy:
	$(MAKE) agent-image-push
	$(MAKE) agent-deployment-refresh

agent-deployment-refresh:
	$(MAKE) agent-deployment-delete
	$(MAKE) agent-deployment-create

agent-logs:
	kubectl logs deployment/agent agent -f

all-deploy:
	$(MAKE) agent-image-push
	$(MAKE) producer-image-push
	$(MAKE) agent-deployment-refresh

kube-registry-secret:
	kubectl delete secret registry-creds || true
	kubectl create secret docker-registry registry-creds --docker-server=https://index.docker.io/v1/ --docker-username=$$DOCKER_USERNAME --docker-password=$$DOCKER_PASSWORD --docker-email=$$DOCKER_EMAIL

kube-envvar-secret:
	kubectl delete secret openai-key || true
	kubectl create secret generic openai-key --from-literal=key=$$OPENAI_KEY

docker-login:
	docker login -u $$DOCKER_USERNAME -p $$DOCKER_PASSWORD
