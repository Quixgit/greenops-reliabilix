# Sign-in page: identity provider setup

The page at `/login` is branded and ours; the secrets are not. Passwords and second factors are typed only on the
identity provider's own page (Auth0, ADR-0004).

| Control on the page | What it does | What to configure in Auth0 |
|---|---|---|
| Continue with Google | `/auth/login?connection=google-oauth2` | Enable the **google-oauth2** social connection for the application |
| Continue with GitHub | `/auth/login?connection=github` | Enable the **github** social connection for the application |
| E-mail + Continue | `/auth/login?login_hint=<e-mail>`: the provider page opens with the address filled in, asks for the password and, when enabled, the second factor | A database connection (default name `Username-Password-Authentication`; set `AUTH0_DB_CONNECTION` if yours differs) |
| Forgot your password? | `POST /api/auth/forgot-password` asks Auth0 to e-mail a reset link (`/dbconnections/change_password`) | Same database connection; set up the e-mail provider in Auth0 |
| Two-step verification | Asked by Auth0 after the password when the policy requires it | Security > Multi-factor Auth: enable a factor (Authenticator app / WebAuthn) and set the policy (for example "Always" or adaptive) |

Notes
- The recovery answer is the same for any address (no account enumeration) and is limited to 5 requests a minute per client address.
- `returnTo` is accepted only as an app-relative path; anything else becomes `/overview`.
- In the development environment (`DEV_AUTH=true`) the buttons are disabled and a notice says so; the page can be viewed at `/login?preview`.
- A social login with an e-mail Auth0 has not verified is refused by the API (`email_verified` is required to accept invitations).
