# One-time AWS setup for GitHub Actions OIDC

The deploy workflow authenticates with **OIDC**, so there are no AWS access keys
stored in GitHub. This is the recommended path: credentials are short-lived and
scoped to the role below.

## 1. Create the OIDC identity provider

Only needs doing once per AWS account. Multiple roles can share it.

```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com \
  --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1
```

## 2. Create the deploy role

The trust policy below pins **both** the audience and the subject. The `sub`
condition is the important one — without it, any repository in any GitHub org
could assume this role.

```bash
aws iam create-role \
  --role-name weather-watcher-deploy \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": { "Federated": "arn:aws:iam::ACCOUNT_ID:oidc-provider/token.actions.githubusercontent.com" },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
          "token.actions.githubusercontent.com:sub": "repo:YOUR_GITHUB_USER/weather-watcher:environment:production"
        }
      }
    }]
  }'
```

The `sub` value is `<owner>/<repo>:<ref>:<environment>`. Because the workflow
sets `environment: production`, the environment segment must be present.

## 3. Attach least-privilege permissions

```bash
aws iam put-role-policy \
  --role-name weather-watcher-deploy \
  --policy-name deploy \
  --policy-document file://deploy-policy.json
```

`deploy-policy.json` should grant only what `terraform apply`, `ecr
get-login-password` and `apprunner start-deployment / describe-service`
actually call — nothing broader.

## 4. Add the role ARN as a repository secret

In GitHub: **Settings → Secrets and variables → Actions → New repository
secret**

- Name: `AWS_DEPLOY_ROLE_ARN`
- Value: the ARN from step 2

## Verify

Push to `main` and watch the **Deploy to AWS** job. If the role assumption is
rejected, the `sub` in the trust policy is almost always the mismatch.