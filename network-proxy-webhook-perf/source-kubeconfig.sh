# sudo copy to /run, then source this script
cp /mnt/disks/master-pd/srv/kubernetes/kube-controller-manager/kubeconfig /home/xuchao
token=$(sudo cat /etc/srv/kubernetes/known_tokens.csv | awk 'BEGIN {FS=","}; FNR == 1 {print $1}')
echo ${token}
sed -i "s/token: .*$/token: ${token}/g" /home/xuchao/kubeconfig
export KUBECONFIG=/home/xuchao/kubeconfig
