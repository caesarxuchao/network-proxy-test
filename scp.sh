tar -czvf network-proxy-webhook-perf.tar.gz network-proxy-webhook-perf/
gcloud compute scp ./network-proxy-webhook-perf.tar.gz xuchao@kubernetes-master:/home/xuchao --project "chao1-149704" --zone "us-central1-b"
