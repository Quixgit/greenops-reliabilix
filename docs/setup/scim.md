# Connecting your identity provider with SCIM

1. In Reliabilix, Settings > API keys, create a key with access **Identity provider (SCIM)**. It is shown once.
2. In your provider (Okta, Microsoft Entra, others), add a SCIM 2.0 application:
   - base URL: `https://<your-api-host>/api/v1/scim/v2`
   - authentication: bearer token = the key
   - unique identifier: `userName` (the person's e-mail); supported actions: create, update, deactivate users. Groups are not supported.
3. Send the role in `roles` (viewer, engineer, admin or billing). Without it a person is a viewer. Owners are never created or removed by SCIM.
4. A person pushed from the directory becomes a member the first time they sign in with the same, verified e-mail. Deactivating them in the directory removes their access.

Limits and behaviour: see [ADR-0056](../adr/0056-scim-provisioning.md).
