#!/usr/bin/env python3
import smtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
import argparse


def send_email(smtp_host, smtp_port, from_addr, to_addr, subject, body):
    # Create the email message
    msg = MIMEMultipart()
    msg["From"] = from_addr
    msg["To"] = to_addr
    msg["Subject"] = subject

    # Add body to email
    msg.attach(MIMEText(body, "plain"))

    try:
        print(f"Attempting to connect to {smtp_host}:{smtp_port}...")
        # Create SMTP connection with debugging
        with smtplib.SMTP(smtp_host, smtp_port, timeout=10) as server:
            server.set_debuglevel(1)  # Enable debug output
            print("Connected to server")

            # Send email
            print("Attempting to send message...")
            server.send_message(msg)
            print(f"\nEmail sent successfully!")
            print(f"From: {from_addr}")
            print(f"To: {to_addr}")
            print(f"Subject: {subject}")
            print(f"Body: {body}")
    except ConnectionRefusedError:
        print(
            f"Connection refused - Is the SMTP server running on {smtp_host}:{smtp_port}?"
        )
    except smtplib.SMTPServerDisconnected as e:
        print(f"Server disconnected: {e}")
        print("This might happen if:")
        print("1. The server doesn't accept the connection")
        print("2. The server requires authentication")
        print("3. The server closed the connection unexpectedly")
    except Exception as e:
        print(f"Failed to send email: {e}")
        print(f"Error type: {type(e).__name__}")


def main():
    parser = argparse.ArgumentParser(
        description="Send a test email to a local SMTP server"
    )
    parser.add_argument(
        "--host", default="localhost", help="SMTP server host (default: localhost)"
    )
    parser.add_argument(
        "--port", type=int, default=25, help="SMTP server port (default: 25)"
    )
    parser.add_argument(
        "--from",
        dest="from_addr",
        default="sender@example.com",
        help="From email address",
    )
    parser.add_argument(
        "--to", dest="to_addr", default="recipient@example.com", help="To email address"
    )
    parser.add_argument(
        "--subject", default="Test Subject Word1 Word2", help="Email subject"
    )
    parser.add_argument(
        "--body", default="This is a test email body.", help="Email body"
    )

    args = parser.parse_args()

    send_email(
        args.host, args.port, args.from_addr, args.to_addr, args.subject, args.body
    )


if __name__ == "__main__":
    main()
