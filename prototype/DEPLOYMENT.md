# Insucar prototype — AWS deployment record

## Live endpoints
- End-user app:      http://app.unysolar.com:8080/   (also /login, /register)
- API health:        http://api.unysolar.com:8080/healthz
- Operator console:  http://app.unysolar.com:8080/ops-console-7f3a9c   (hidden path; guesses 404)
- Raw IP:            http://108.129.149.127:8080/

## AWS resources (account 326804802908, region eu-west-1)
- EC2 instance: i-0628af42823122bce (t3.small, Amazon Linux 2023) — runs the Docker Compose stack
  (PostgreSQL/PostGIS + Go API serving both apps).
- Security group: sg-08bfda3ebbaa34e92 (ingress 8080/80/22).
- S3 bucket: insucar-deploy-326804802908 (deploy artifact, private; shipped via presigned URL).
- Route 53 hosted zone: Z06143773JJ0DPRILPDA0 (unysolar.com — already delegated to AWS).
  Records added: app / api / demo .unysolar.com -> 108.129.149.127 (A, TTL 60).

## Verified (all green)
- APIs: health, end-user register (real insert + consent + audit-ledger entry), ANI screen-pop
  lookup, create case, dispatch (real provider + mission + driver + ETA), get case.
- Headless browser: operator UI flow (answer call -> screen-pop populated from DB -> dispatch ->
  provider/driver/ETA) PASSED; screenshot operator-live.png.
- Hidden operator surface: wrong paths (/admin, /ops-console) -> 404; exact obscure path -> 200.
- Reachable through app.unysolar.com.

## What this is / is NOT (honest scope)
- IS: a real, running AWS deployment of the working vertical slice (registration, screen-pop,
  case, dispatch) on real PostgreSQL/PostGIS, reachable on the real domain.
- IS NOT (yet): the full production design. Still to do for production parity —
  * Telephony: live Amazon Connect number + Lex + CCP softphone (needs a claimed number).
  * HA: managed Multi-AZ data (EKS across 3 AZs + Amazon RDS Multi-AZ) — this demo is a SINGLE instance (no failover).
  * TLS: HTTPS via ACM + ALB/CloudFront (currently plain HTTP on :8080).
  * Rust inner-core vault, Amazon Cognito SSO, Spinnaker CD, Pinpoint SMS, real provider sandbox.

## Security follow-ups (do these)
- ROTATE the AWS credentials used: they are ROOT access keys (arn:...:root). Delete them and use a
  scoped IAM user/role. Root keys should not exist.
- Lock down the security group (restrict 22; put the app behind ALB+WAF; TLS only).

## Teardown (stop incurring cost)
    aws ec2 terminate-instances --instance-ids i-0628af42823122bce
    aws ec2 delete-security-group --group-id sg-08bfda3ebbaa34e92   # after instance is gone
    aws s3 rb s3://insucar-deploy-326804802908 --force
    # optionally delete the app/api/demo A records from zone Z06143773JJ0DPRILPDA0
