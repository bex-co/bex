def inner_command(row,source,digest_hex):
    return ['/usr/local/bin/cosign','verify-blob-attestation','--bundle','/bundle.json','--type',row['predicate_type'],
        '--digest',digest_hex,'--digestAlg','sha256','--certificate-identity',
        'https://github.com/bex-co/bex/.github/workflows/native-base-image.yml@refs/heads/main',
        '--certificate-oidc-issuer','https://token.actions.githubusercontent.com','--certificate-github-workflow-sha',source,
        '--use-signed-timestamps','--timeout','5m']
