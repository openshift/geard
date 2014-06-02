/*
Job implementations for core API.  Container jobs control container related actions on a server.
Each request object has a default implementation on Linux via systemd, and a structured response if
necessary.  The Execute() method is separated so that client code and server code can share common
sanity checks.
*/
package jobs
