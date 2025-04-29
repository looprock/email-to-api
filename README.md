# Email Processing Server

A Go-based server application that receives incoming emails, processes them to extract subject and body, and forwards the data as JSON payloads to designated API endpoints.

## Features

- Receives incoming emails through SMTP or webhook endpoints
- Parses email subjects into arrays (space-delimited)
- Extracts email body content
- Maps incoming email addresses to API endpoints using SQLite
- Forwards processed data as JSON payloads
- Configurable routing and endpoint mapping
- Web-based admin interface for managing mappings and viewing logs
- Separate mail and admin servers for better isolation and scalability

## Requirements

- Go 1.20 or higher
- SQLite 3
- Git

## Setup

1. Clone the repository
2. Set up your environment variables (see Configuration section)
3. Run `go mod download` to install dependencies
4. Run database migrations:
   ```bash
   go run scripts/migrate.go
   ```
5. **Create an initial admin user (if none exists):**
   ```bash
   go run scripts/create_admin.go -email=admin@example.com -password=yourpassword
   ```
6. Start both servers (in separate terminals):
   ```bash
   # Terminal 1: Start the mail server
   go run cmd/mailserver/main.go

   # Terminal 2: Start the admin server
   go run cmd/adminserver/main.go
   ```

## Configuration

The application uses environment variables for configuration. Here are the key variables:

```bash
# Admin Server Configuration
ADMIN_SERVER_PORT=8080    # Port for admin interface
ADMIN_SERVER_HOST=localhost # Host for admin interface

# Mail Server Configuration
MAIL_SERVER_PORT=25       # Port for mail server
MAIL_SERVER_HOST=localhost # Host for mail server
EMAIL_RECEIVE_METHOD=smtp # Email receiving method (smtp/webhook)
MAILREADER_SMTP_PORT=2525 # Port for SMTP server (use >1024 for non-root)
MAILREADER_SMTP_HOST=localhost # Host for SMTP server

# Common Configuration
DB_PATH=./data/mailreader.db  # Shared database used by both servers
```

You can set these variables in your environment or create a `.env` file.

### Database Sharing

Both the mail server and admin server use the same SQLite database file specified by `DB_PATH`. This is by design as:
- The mail server needs to read email-to-API mappings
- The admin interface manages these mappings
- Both servers contribute to and read from the same log entries

When deploying in a production environment:
- Ensure both servers have proper read/write permissions to the database file
- Consider using a more robust database system if high concurrency is needed
- Make sure the database file is in a location accessible to both servers if deployed separately

## Environment Variables

Required:
- `MAILREADER_DOMAIN` - Domain for generated email addresses

Optional:
- `DB_PATH` - Path to SQLite database file (default: mailreader.db)
- `MAILREADER_SMTP_HOST` - SMTP server host (default: 0.0.0.0)
- `MAILREADER_SMTP_PORT` - SMTP server port (default: 25)
- `MAILREADER_MAX_EMAIL_SIZE` - Maximum email size in bytes (default: 10MB)
- `MAILREADER_MAX_RETRIES` - Maximum retry attempts for failed API calls (default: 10)
- `MAILREADER_RETRY_DELAY` - Delay between retries in seconds (default: 5)
- `MAILREADER_RECEIVE_METHOD` - Email receiving method: "smtp" or "api" (default: smtp)
- `ADMIN_SERVER_HOST` - Admin interface host (default: 0.0.0.0)
- `ADMIN_SERVER_PORT` - Admin interface port (default: 8080)
- `MAIL_SERVER_HOST` - Mail processing service host (default: localhost)
- `MAIL_SERVER_PORT` - Mail processing service port (default: 25)

Mailgun Configuration (optional, required for sending registration emails):
- `MAILGUN_API_KEY` - Your Mailgun API key
- `MAILGUN_DOMAIN` - Your Mailgun domain (required if MAILGUN_API_KEY is set)
- `MAILGUN_FROM_ADDRESS` - Sender email address (required if MAILGUN_API_KEY is set)
- `SITE_DOMAIN` - Your site's domain (required if MAILGUN_API_KEY is set)

## Usage

### Running the Servers

1. First, run the database migrations:
   ```bash
   go run scripts/migrate.go
   ```

2. **Create an initial admin user (if none exists):**
   ```bash
   go run scripts/create_admin.go -email=admin@example.com -password=yourpassword
   ```
   - This will create an admin user with the specified email and password.
   - You can create additional admin or regular users from the web interface after logging in.

3. Start the mail server:
   ```bash
   go run cmd/mailserver/main.go
   ```
   This will start:
   - SMTP server on `MAILREADER_SMTP_HOST:MAILREADER_SMTP_PORT`
   - Mail processing service on `MAIL_SERVER_HOST:MAIL_SERVER_PORT`

4. Start the admin server (in a separate terminal):
   ```bash
   go run cmd/adminserver/main.go
   ```
   This will start:
   - Admin interface on `ADMIN_SERVER_HOST:ADMIN_SERVER_PORT`

### Accessing the Admin Interface

1. Open your browser and go to `http://localhost:8080/login`
2. Log in using the admin email and password you created with the script above.
3. Once logged in, you can:
   - Create new users (admin or regular)
   - Manage email-to-API mappings
   - View logs

**Note:** The previous token-based login (`?token=your_secret_key`) is no longer used. The system now uses email/password authentication for all users.

### Managing Email Mappings

In the admin interface, you can:
- View all email-to-API mappings
- Add new mappings
- Delete existing mappings
- Monitor mapping status

### Viewing Logs

The logs section shows:
- Last 100 email processing attempts
- Processing status (success/error)
- Error messages (if any)
- Timestamps and email details

### Sending Emails

Configure your email client to use:
- SMTP server: localhost (or your server address)
- Port: 2525 (or your configured SMTP_PORT)
- No authentication required for testing

The server will:
1. Receive the email
2. Parse the subject into an array (space-delimited)
3. Extract the body
4. Forward to the configured API endpoint as JSON:
   ```json
   {
     "subjects": ["word1", "word2", "..."],
     "body": "email body content"
   }
   ```

## Project Structure

```
.
├── cmd/                    # Application entry points
│   ├── mailserver/        # Mail server application
│   └── adminserver/       # Admin server application
├── internal/              # Private application code
│   ├── admin/            # Admin interface
│   ├── config/           # Configuration handling
│   ├── email/            # Email processing logic
│   ├── database/         # Database operations
│   └── api/              # API integration
├── pkg/                   # Public libraries
├── migrations/            # Database migrations
└── scripts/              # Utility scripts
```

## Security Notes

- Use a strong API key for production
- Consider running behind a reverse proxy
- Configure firewall rules for SMTP and admin ports
- Use SSL/TLS in production
- The split server architecture allows for better security isolation between mail and admin functions

## License

MIT License 