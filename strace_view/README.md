# Strace Viewer

It's a tool to view strace output in a web browser.


# Work Notes

```shell
strace -ttt -f -s 256 -a 100 -T --stack-traces --decode-fds=all ./concurrency 
```

Strace Postgres
```shell
strace -t -f sudo -u postgres /usr/lib/postgresql/14/bin/postgres -D /var/lib/postgresql/14/main -c config_file=/etc/postgresql/14/main/postgresql.conf
```

