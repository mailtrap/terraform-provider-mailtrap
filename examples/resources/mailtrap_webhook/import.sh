# Import a webhook by its numeric ID. The signing_secret cannot be recovered
# on import; it is only returned by the API when the webhook is created.
terraform import mailtrap_webhook.example 12345
