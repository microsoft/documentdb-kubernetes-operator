FROM gcr.io/distroless/base-debian12:latest

COPY --from=postgresql:17 /usr/share/postgresql-common/ /bin

ENV PATH="$PATH:/bin/postgresql-common" 

ENTRYPOINT [
    "pg_wrapper"
          --slot
      wal_replica
      --compress
      0
      --directory
      /var/lib/postgresql/wal
      --dbname
      postgres://streaming_replica@cluster-example-rw/postgres?sslmode=verify-full&sslrootcert=/var/lib/postgresql/rootcert/ca.crt&sslcert=/var/lib/postgresql/cert/tls.crt&sslkey=/var/lib/postgresql/cert/tls.key
      --verbose
    ]
