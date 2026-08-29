# Deliverability checklist

Deliverability is a deployment contract, not an application-code claim. Before the
first production message:

1. verify the sender domain as an Amazon SES domain identity in the selected region;
2. publish the exact three 2048-bit Easy DKIM CNAME records returned by SES;
3. configure a dedicated subdomain as custom MAIL FROM and publish the exact
   region-specific SES MX plus `v=spf1 include:amazonses.com ~all` there;
4. publish DMARC with monitoring policy `p=none`; add `rua` only
   after a real aggregate-report mailbox or processor exists, then stage
   `quarantine` and `reject` after verifying every legitimate sender;
5. verify a received message reports aligned `spf=pass`, `dkim=pass`, and
   `dmarc=pass`, uses TLS, and follows RFC 5322;
6. attach an SES configuration set for delivery, hard-bounce, complaint, reject,
   and delay events, and enable account-level suppression;
7. alarm before hard bounce reaches 5% or complaint reaches 0.1%, and monitor Gmail
   Postmaster Tools with a target spam rate below 0.3%.

Do not create a second SPF record. Inventory Google Workspace and any other sender,
then merge every authorized source into the single applicable SPF TXT record.
Transactional security mail uses a stable sender and separate stream from any future
marketing mail. Marketing or subscription mail additionally requires one-click and
visible unsubscribe controls; Cerberus invitation mail is transactional.

Do not invent DKIM selectors or a region-specific MX. Apply the exact values returned
by the selected provider and keep hosted-zone identifiers in private infrastructure.

Sources: [Amazon SES authentication](https://docs.aws.amazon.com/ses/latest/dg/email-authentication-methods.html),
[Amazon SES DMARC](https://docs.aws.amazon.com/ses/latest/dg/send-email-authentication-dmarc.html),
[Amazon SES reputation alarms](https://docs.aws.amazon.com/ses/latest/dg/reputationdashboard-cloudwatch-alarm.html),
and [Gmail sender guidelines](https://support.google.com/mail/answer/81126).
