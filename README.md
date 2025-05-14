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
   go run scripts/migrate/migrate.go
   ```
5. **Create an initial admin user (if none exists):**
   ```bash
   # There's a hash utility to create password hashes for manual insertion
   go run scripts/hash/hash.go -password=yourpassword
   # Use the generated hash to create an admin user in the database
   ```
6. Start both servers (in separate terminals):
   ```bash
   # Terminal 1: Start the mail server
   go run cmd/mailserver/main.go

   # Terminal 2: Start the admin server
   go run cmd/adminserver/main.go
   ```

## Configuration

The application uses [Viper](https://github.com/spf13/viper) for configuration management, supporting both YAML configuration files and environment variables. Configuration values are loaded in the following order (each item overrides the previous ones):

1. Default values
2. Configuration file
3. Environment variables

### Configuration File

The application looks for a `config.yaml` file in the following locations:
- Current directory (`./config.yaml`)
- Home directory (`$HOME/.emailtoapi/config.yaml`)
- System directory (`/etc/emailtoapi/config.yaml`)

You can copy the provided `config.example.yaml` as a starting point:
```bash
cp config.example.yaml config.yaml
```

Example configuration file:
```yaml
# Database Configuration
database:
  driver: sqlite  # sqlite or postgres
  # SQLite configuration
  path: ./data/emailtoapi.db
  # PostgreSQL configuration (when driver is postgres)
  host: localhost
  port: 5432
  user: postgres
  password: ""
  name: emailtoapi
  sslmode: disable

# Admin Server Configuration
adminserver:
  host: 0.0.0.0
  port: 8080

# Mail Server Configuration
mailserver:
  domain: example.com  # Domain for generated email addresses
  receivemethod: smtp  # smtp or webhook
  maxemailsize: 10485760  # 10MB in bytes
  maxretries: 10
  retrydelay: 5
  smtphost: 0.0.0.0
  smtpport: 25

# Mailgun Configuration (optional)
mailgun:
  apikey: ""
  domain: ""
  fromaddress: ""
  site_domain: example.com # Domain for registration link if mailgun is used
```

### Environment Variables

All configuration options can also be set via environment variables. The application uses the prefix `EMAILTOAPI_` and converts dots to underscores. For example:

```bash
# Database Configuration
EMAILTOAPI_DATABASE_DRIVER=postgres
EMAILTOAPI_DATABASE_HOST=localhost
EMAILTOAPI_DATABASE_PORT=5432
EMAILTOAPI_DATABASE_USER=postgres
EMAILTOAPI_DATABASE_PASSWORD=secret
EMAILTOAPI_DATABASE_NAME=emailtoapi
EMAILTOAPI_DATABASE_SSLMODE=disable

# Admin Server Configuration
EMAILTOAPI_ADMINSERVER_HOST=0.0.0.0
EMAILTOAPI_ADMINSERVER_PORT=8080

# Mail Server Configuration
EMAILTOAPI_MAILSERVER_DOMAIN=example.com
EMAILTOAPI_MAILSERVER_SMTPPORT=2525
```

### Legacy Environment Variables

For backward compatibility, the following legacy environment variables are still supported:

```bash
# Database (old format)
DB_DRIVER=sqlite
DB_PATH=./data/mailreader.db
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=
DB_NAME=emailtoapi
DB_SSLMODE=disable

# Mail Server (old format)
MAILREADER_DOMAIN=example.com
MAILREADER_SMTP_HOST=0.0.0.0
MAILREADER_SMTP_PORT=2525
MAILREADER_MAX_EMAIL_SIZE=10485760
MAILREADER_MAX_RETRIES=10
MAILREADER_RETRY_DELAY=5
MAILREADER_RECEIVE_METHOD=smtp

# Mailgun (old format)
MAILGUN_API_KEY=
MAILGUN_DOMAIN=
MAILGUN_FROM_ADDRESS=
```

These legacy variables will be mapped to their new counterparts automatically, but it's recommended to use the new format for consistency.

## Usage

### Running the Servers

1. First, run the database migrations:
   ```bash
   # The migrations will run automatically when starting either server
   # You can also run them manually using the migrate command:
   go run scripts/migrate/migrate.go
   ```

   The migrations are idempotent and safe to run multiple times. They will:
   - Create necessary database tables if they don't exist
   - Apply any pending migrations in order
   - Skip migrations that have already been applied
   - Work with both SQLite and PostgreSQL databases

2. **Create an initial admin user (if none exists):**
   ```bash
   # Create a password hash first
   go run scripts/hash/hash.go -password=yourpassword
   # Then use the hash to manually create an admin user in the database
   # or use the registration page to create a user, then update their role in the database
   ```
   - You can create additional admin or regular users from the web interface after logging in.

3. Start the mail server:
   ```bash
   go run cmd/mailserver/main.go
   ```
   This will start:
   - SMTP server on the configured `smtphost:smtpport`
   - Mail processing service

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
- Port: 25 (or your configured smtpport)
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
│   ├── adminserver/       # Admin server application
│   └── server/            # Combined server application
├── internal/              # Private application code
│   ├── admin/            # Admin interface
│   ├── config/           # Configuration handling
│   ├── email/            # Email processing logic
│   ├── database/         # Database operations
│   └── api/              # API integration
├── pkg/                   # Public libraries
├── migrations/            # Database migrations
└── scripts/              # Utility scripts
    ├── migrate/          # Database migration script
    └── hash/             # Password hashing utility
```

## Security Notes

- Use a strong API key for production
- Consider running behind a reverse proxy
- Configure firewall rules for SMTP and admin ports
- Use SSL/TLS in production
- The split server architecture allows for better security isolation between mail and admin functions

## License

MIT License 