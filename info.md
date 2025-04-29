# Admin functionality

## admin user
- username and password authentication for the admin interface instead of using an key in the URL 
- The admin interface should require a domain to be passed in as an environment variable. 
- An admin password should be pulled from an environment variable, and the admin login should provide a page for creating users. 
- creating a user involves entering an email address functioning as a username and a role assignable as either admin or user. Once a user is created, an email is sent through mailgun providing them with a registration link where they can create a password and log in

## standard user
- There is no signup page, an administrator needs to register a user
- The user has the ability to create new mappings similar to now those function currently, however instead of defining the email address, an email address consisting of a random string for the username and the domain passed in as an environment variable will be auto-assigned to the api endpoing
- the admin user also supports the same capabilities as a user


# Email server functionality

- silently drop any unmapped emails