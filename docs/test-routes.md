# Testing Routes

To verify routes are working, test these endpoints:

1. Health check:
   ```
   GET http://localhost:8080/health
   ```

2. Admin routes:
   ```
   GET http://localhost:8080/api/admin/therapists/pending
   GET http://localhost:8080/api/admin/therapists/approved
   ```

If you get 404, make sure:
- Server has been restarted after code changes
- No compilation errors in terminal
- Routes are logged on server startup

