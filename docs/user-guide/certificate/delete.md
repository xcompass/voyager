## Deleting a Certificate
Deleting a Kubernetes Certificate object will only delete the certificate CRD from kubernetes.
It will not delete the obtained certificate and user account secret from kubernetes. User have to manually delete
the secrets for removing those.

### Delete Certificate
```
kubectl delete certificate test-cert
```

**Delete Obtained Lets Encript Certificate**
```
kubectl delete secret cert-test-cert
```

**Delete Lets Encrypt User Account Secret**
```
kubectl delete secret acme-test-cert
```