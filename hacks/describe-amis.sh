export AWS_PAGER=""
for region in $(aws ec2 describe-regions --output text --query 'Regions[].RegionName'); do
results=$(aws ec2 describe-images --region "$region" --owners self amazon --filters "Name=name,Values=rhel-ai*" --output text --query "Images[].[ImageId,Name]")
if [ ! -z "$results" ]; then
    echo "$results" | while IFS=$'\t' read -r ami_id name; do
    echo "ResourceType: AMI"
    echo "ResourceID: $ami_id"
    echo "ResourceName: $name"
    echo "Region: $region"
    echo ""
    done
fi
done
