# Strace Viewer

It's a tool to view strace output in a web browser.


# Work Notes

```shell
strace -ttt -f -s 256 -a 100 -T --stack-traces --decode-fds=all ./concurrency 
```

