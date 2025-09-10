FROM postgres:17 AS pg
FROM gcr.io/distroless/base:latest

COPY --from=pg /usr/lib/postgresql/17/bin/pg_receivewal /bin/pg_receivewal

# Copy libraries
COPY --from=pg /usr/lib/x86_64-linux-gnu/libpq.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/libzstd.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/liblz4.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/libz.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/libgssapi_krb5.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/libldap.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/libxxhash.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/libkrb5.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/libk5crypto.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/libcom_err.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/libkrb5support.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/liblber.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/libsasl2.so.* /usr/lib/x86_64-linux-gnu
COPY --from=pg /usr/lib/x86_64-linux-gnu/libkeyutils.so.* /usr/lib/x86_64-linux-gnu


ENV PATH="$PATH:/bin/postgresql-common" 

ENTRYPOINT ["/bin/pg_receivewal",\
            "--slot", "wal_replica",\
            "--compress", "0",\
            "--directory", "/var/lib/postgresql/wal",\
            "--dbname", "postgres://streaming_replica@cluster-example-rw/postgres?sslmode=verify-full&sslrootcert=/var/lib/postgresql/rootcert/ca.crt&sslcert=/var/lib/postgresql/cert/tls.crt&sslkey=/var/lib/postgresql/cert/tls.key",\
            "--verbose"]
