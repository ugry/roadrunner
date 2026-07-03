# Insucar CI/CD — deployed state (Jenkins + Spinnaker on EKS)

Deployed to AWS account 326804802908, region eu-west-1.

## Cluster
- EKS cluster: `insucar` (k8s 1.30), managed nodegroup `ng-standard` = 2x t3.xlarge.
- Addons: aws-ebs-csi-driver (IRSA role AmazonEKS_EBS_CSI_DriverRole), metrics-server, coredns, vpc-cni.
- Default StorageClass: gp2.
- kubeconfig: `aws eks update-kubeconfig --name insucar --region eu-west-1`.

## Jenkins (namespace: jenkins)
- Install: Helm chart `jenkins/jenkins` with `ci/jenkins-values.yaml` (Config-as-Code).
- URL: http://a69a0dc446e674657ac3fae06d8dd559-1651454478.eu-west-1.elb.amazonaws.com:8080
- Login: admin / InsucarAdmin!2026   (CHANGE THIS)
- JCasC created: `github-pat` credential + seeded pipeline job `insucar-ci`
  (pulls this repo, runs `ci/Jenkinsfile`).
- Plugins: defaults + workflow-job, pipeline-stage-view, docker-workflow, job-dsl, credentials,
  credentials-binding, matrix-auth.

## Spinnaker (namespace: spinnaker; operator in spinnaker-operator)
- Install: spinnaker-operator (cluster mode) + `SpinnakerService` (v1.36.1).
- Services running (OK): clouddriver, orca, gate, deck, front50, rosco, igor, echo, redis.
- Deck (UI):  http://a4977860e39434f278d0b4dedbcd4bb5-449340997.eu-west-1.elb.amazonaws.com
- Gate (API): http://afac25beae62d4f0cab340b254e5e6f2-1288246793.eu-west-1.elb.amazonaws.com  (/health = UP)
- Persistence: S3 bucket `insucar-spinnaker-326804802908` (Front50).
- CI integration: igor lists Jenkins master `insucar-jenkins` (verified via Gate /v3/builds).
- Kubernetes deploy provider: disabled at bring-up; add a SpinnakerAccount (in-cluster SA) next.

## Flow
git push -> Jenkins `insucar-ci` (build/test Go, docker build, push ECR) -> triggers Spinnaker
pipeline (bake -> dev -> manual judgment -> UAT -> canary -> product_owner judgment -> prod).

## Security follow-ups (do these)
- ROTATE the AWS root keys used (they were inlined into the SpinnakerService s3 config in-cluster
  and used for the CLI). Replace with an IAM user or IRSA.
- Rotate the GitHub PAT stored in the `github-pat` k8s secret.
- Put SSO/OIDC (fiat + gate) in front of Spinnaker and Jenkins; restrict the LoadBalancers (SG/WAF);
  serve over TLS.

## Teardown (stop cost — EKS + LBs are the expensive parts)
    helm -n jenkins uninstall jenkins
    kubectl delete -n spinnaker spinnakerservice spinnaker
    kubectl delete ns spinnaker spinnaker-operator jenkins
    eksctl delete cluster --name insucar --region eu-west-1   # deletes cluster + nodegroup + VPC
    aws s3 rb s3://insucar-spinnaker-326804802908 --force
    # also terminate the earlier prototype EC2 (see prototype/DEPLOYMENT.md) if not needed
