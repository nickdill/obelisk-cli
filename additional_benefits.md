Obelisk deployments allow metrics/tracking at the webserver level, something nobody else does.
DDOS protection
nginx logging remotely etc

Deployments / Brainstorming
wrapper on top of aws optionally:
- enter domain, give dns records, we forward to route53
- spin up EC2 instances
- push/pull from ECR
- make cloudfront distros
- make s3 buckets

