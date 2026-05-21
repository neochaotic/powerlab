# Mongo

MongoDB is a document-oriented NoSQL database server. This instance is a headless backend for other apps. Connect on port 27017 with any MongoDB-compatible driver or tool (mongosh, Compass, etc.).

**Database port:** MongoDB is a backend service — connect with `mongosh --host <host> --port 27017`. Credentials are set via `MONGO_INITDB_ROOT_USERNAME` and `MONGO_INITDB_ROOT_PASSWORD` environment variables.

**Data:** `/DATA/PowerLabAppData/mongo/` — BSON data files, journals, and indexes.
