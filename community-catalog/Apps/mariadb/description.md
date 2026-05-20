# MariaDB

MariaDB is a community-developed MySQL-compatible relational database. This instance runs as a headless backend for other apps. Connect with any MySQL-compatible client on port 3306.

**Database port:** MariaDB is a backend service — connect with `mysql -h <host> -P 3306 -u root -p`. Root password is set via the `MYSQL_ROOT_PASSWORD` environment variable.

**Data:** `/DATA/PowerLabAppData/mariadb/` — Database files, binlogs, and InnoDB tablespace.
