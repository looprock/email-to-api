# Test Scripts

This directory contains test scripts for sending test emails to verify the email receiving functionality.

## Available Scripts

### Python Version (`send_test_email.py`)

A Python script that sends a test email using SMTP.

Usage:
```bash
python send_test_email.py [options]
```

Options:
- `--host`: SMTP server host (default: localhost)
- `--port`: SMTP server port (default: 25)
- `--from`: Sender email address (default: sender@example.com)
- `--to`: Recipient email address (default: foo@localhost.localdomain)
- `--subject`: Email subject (default: "Test Subject Word1 Word2")
- `--body`: Email body (default: "This is a test email body.")

### Go Version (`send_test_email.go`)

A Go script that provides the same functionality as the Python version.

Usage:
```bash
go run send_test_email.go [options]
```

Options:
- `-host`: SMTP server host (default: localhost)
- `-port`: SMTP server port (default: 25)
- `-from`: Sender email address (default: sender@example.com)
- `-to`: Recipient email address (default: foo@localhost.localdomain)
- `-subject`: Email subject (default: "Test Subject Word1 Word2")
- `-body`: Email body (default: "This is a test email body.")

## Example Usage

1. Start the example-api server in one terminal:
```bash
cd ../example-api
go run cmd/server/main.go
```

2. In another terminal, send a test email using either script:

Python version:
```bash
python send_test_email.py --subject "Test Email" --body "Hello from Python!"
```

Go version:
```bash
go run send_test_email.go -subject "Test Email" -body "Hello from Go!"
```

## Notes

- The scripts default to connecting to a local SMTP server on port 25
- Make sure the example-api server is running before sending test emails
- The scripts provide detailed logging of the email sending process
- Both scripts handle connection errors and provide helpful error messages
- The email subject and body can be customized via command line arguments 