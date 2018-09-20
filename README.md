# System V Message Queue Exporter for Prometheus
### Why?
Tragically, Prometheus's node_exporter does not have built in support for monitoring message queues and the C wrappers aren't built-in to the sys/unix standard library package for msgctl

I needed a way to monitor message queues between our Nagios instance and our MySQL db

### How?
I use the output of the command "ipcs -q" on the host. So, it might not work for you. It's only confirmed to work on RHEL 6 & 7.