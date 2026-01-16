# cert-manager-webhook-active24cz
- For [cert-manager](https://cert-manager.io/docs/)
- Using [Active24](https://rest.active24.cz/v2/docs) /
[Websupport REST API v2.0](https://rest.websupport.sk/v2/docs).
- Inspired by
[`websupport`](https://github.com/bratislava/cert-manager-webhook-websupport)
and [`active24`](https://github.com/rkosegi/cert-manager-webhook-active24) webhooks
- Developed at [Merica s.r.o.](https://merica.cz)

## Usage
1. Install the [chart](chart/)
```bash
helm -n cert-manager upgrade -i cert-manager-webhook-active24cz chart/
```

2. Create a secret with your credentials
```bash
kubectl -n cert-manager create secret generic active24cz \
  --from-literal apiKey="$API_KEY" \
  --from-literal apiSecret="$API_SECRET" \
  --from-literal serviceId="$SERVICE_ID"
```

3. Create an issuer
```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: mail@your.tld
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - selector:
        dnsZones:
          - your.tld
      dns01:
        webhook:
          groupName: acme.merica.cz
          solverName: active24cz
          config:
            apiKeySecretRef:
              name: active24cz
```

## Testing
1. Create a secret with your credentials
```bash
kubectl create secret generic active24cz \
  --from-literal apiKey="$API_KEY" \
  --from-literal apiSecret="$API_SECRET" \
  --from-literal serviceId="$SERVICE_ID" \
  --dry-run=client --output yaml > testdata/secret.yaml
```

2. Run the tests
```bash
make test TEST_ZONE_NAME=your.tld.
```
