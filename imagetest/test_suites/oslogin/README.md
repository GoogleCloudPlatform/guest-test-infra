# Overview
Having a separate test organization set up for this test suite is highly recommended.
This is because having a separate test org:
1. Provides a clean environment. You may have policies set in your current organization that
could affect the test results.
2. Allows for easy creation of the test users, the setup for which is specified later in this document.
Otherwise, users would have to create their own by other means.

## Test Setup
This test uses multiple secrets to store the test users that it uses in order to run the test.
These secrets are as follows:

- `normal-user` - the email for the normal SSH user.
- `admin-user` - the email for the normal SSH sudo user.
- `normal-user-ssh-key` - the private SSH key for `normal-user`
- `admin-user-ssh-key` - the private SSH key for `admin-user`

For the 2FA users, there are 3 for each of normal and admin 2FA users, of which
there are 5.
- `normal-2fa-user` - the email for the normal 2FA SSH user
- `normal-2fa-ssh-key` - the private SSH key for `normal-2fa-user`
- `normal-2fa-key` - the 2FA secret for `normal-2fa-user`

- `admin-2fa-user` - the email for the admin 2FA SSH user
- `admin-2fa-ssh-key` - the private SSH key for `admin-2fa-user`
- `admin-2fa-key` - the 2FA secret for `admin-2fa-user`

For every 2FA user past the first, a number starting from 1 is appended at the
end. For example, the secrets for the next normal 2FA SSH user should be named
`normal-2fa-user-1`, `normal-2fa-ssh-key-1`, and `normal-2fa-key-1`, and the
next after should be `normal-2fa-user-2`, etc.

As a consequence, users of this test suite must be able to set up and provide 12 test users.
Each of these test users should have permissions set up according to the
[OSLogin setup](https://cloud.google.com/compute/docs/oslogin/set-up-oslogin#configure_users) page.
Note that the admin users should have the `roles/compute.osAdminLogin` permission.

## User Setup
In order to set up each of the users for the test, their OSLogin profiles must be set up with a
permanent public SSH key. This can be accomplished by performing the following:
1. [Create a VM with OSLogin enabled](https://cloud.google.com/compute/docs/oslogin/set-up-oslogin#enable_os_login_during_vm_creation).
2. [SSH](https://cloud.google.com/compute/docs/connect/standard-ssh) to the new VM as one of your test users.
3. [Create an SSH key pair](https://cloud.google.com/compute/docs/connect/create-ssh-keys#create_an_ssh_key_pair).
4. Add the SSH public key to the user's OSLogin Profile.
```
gcloud compute os-login ssh-keys add --key-file=/path/to/keyfile.pub
```
5. Copy and paste the contents of the private key file into the appropriate secret, or use the `gcloud` CLI.
For example, you can add the private SSH Key for `normal-user` by calling the following:
```
gcloud secrets create normal-user-ssh-key --data-file=/path/to/keyfile
```
6. Repeat steps 2-5 for each of the test users.

### Two-factor Authentication
As the test requires the 2FA secrets, when setting up the 2FA users, it is important that you acquire the secret before
completing the 2FA setup. The steps are as follows:

1. While logged in as the user, visit the [Security](https://myaccount.google.com/security) page.
2. Set up Google Authenticator.
3. Scan the barcode, but ***do not hit the next button***.
4. Click the linked text saying that you can't scan the barcode, which reveals your 2FA secret.
Save this secret into the appropriate secret in GCP's Secret Manager.
5. Finish the 2FA setup.
