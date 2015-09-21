require 'aws-sdk'
require 'dotenv'

Dotenv.load

SUCCESS_STATS = [:create_complete, :update_complete, :update_rollback_complete]
FAILED_STATS = [:create_failed, :update_failed]
DEFAULT_RECIPES = [
  "apt",
  "ark",
  "elasticsearch",
  "elasticsearch::aws",
  "elasticsearch::proxy",
  "java",
  "layer-custom::esplugins",
  "layer-custom::allocation-awareness",
  "layer-custom::esmonit",
  "layer-custom::cloudwatch-custom"
].join(",")

def opsworks
  Aws::OpsWorks::Client.new({region: get_required("AWS_REGION")})
end

def wait_for_cf_stack_op_to_finish
  stats = cfm.describe_stacks({stack_name: stack_name}).stacks[0].stack_status.downcase.to_sym
  puts "[Stack: #{stack_name}]: current status: #{stats}"

  while !SUCCESS_STATS.include?(stats)
    sleep 15
    stats = cfm.describe_stacks({stack_name: stack_name}).stacks[0].stack_status.downcase.to_sym
    raise "Resource stack update failed!" if FAILED_STATS.include?(stats)
    puts "[Stack: #{stack_name}]: current status: #{stats}"
  end
end

def cf_query_output(key)
  output = cf_stack.outputs.find { |o| o.output_key == key }
  output && output.output_value
end

def instance_online?(instance_id)
  response = opsworks.describe_instances(instance_ids: [instance_id])
  response[:instances].first[:status] == "online"
end

def instance_status(instance_id)
  begin
    response = opsworks.describe_instances(instance_ids: [instance_id])
  rescue Aws::OpsWorks::Errors::ResourceNotFoundException
    return "nonexistent"
  end
  response[:instances].first[:status].tap do |status|
    raise "Instance #{instance_id} has a failed status #{status}" if status =~ /fail|error/i
  end
end

def wait_for_instance(instance_id, status)
  while (ins_status = instance_status(instance_id)) != status
    puts "[Instance #{instance_id}] waiting for instance to become #{status}. Current status: #{ins_status}"
    sleep 10
  end
end

def all_availability_zones
  ec2 = Aws::EC2::Client.new
  ec2.describe_availability_zones["availability_zones"].map(&:zone_name)
end

def get_all_instances(layer_id)
  response = opsworks.describe_instances({layer_id: layer_id})
  response[:instances]
end

def attach_ebs_volumes(instance_id, volume_ids)
  volume_ids.each do |volume_id|
    puts "Attaching EBS volume #{volume_id} to instance #{instance_id}"
    opsworks.assign_volume({volume_id: volume_id, instance_id: instance_id})
  end
end

def detach_ebs_volumes(instance_id)
  response = opsworks.describe_volumes(instance_id: instance_id)
  volume_ids = response[:volumes].map { |v| v[:volume_id] }
  volume_ids.each do |volume_id|
    puts "Detaching EBS volume #{volume_id} from instance #{instance_id}"
    opsworks.unassign_volume(volume_id: volume_id)
  end

  volume_ids
end

def create_instance(stack_id, layer_id, az)
  opsworks.create_instance({stack_id: stack_id,
                            layer_ids: [layer_id],
                            instance_type: ENV['INSTANCE_TYPE'] || 'c3.large',
                            install_updates_on_boot: !ENV['SKIP_INSTANCE_PACKAGE_UPDATES'],
                            availability_zone: az})
end

def update_instances(stack_id, layer_id, count)
  azs = all_availability_zones
  existing_instances = get_all_instances(layer_id)
  count_to_create = count - existing_instances.size
  new_instances = (1..count_to_create).map do |i|
    instance = create_instance(stack_id, layer_id, azs[(existing_instances.size + i) % azs.size])
    puts "Created instance, id: #{instance[:instance_id]}, starting the instance now."
    opsworks.start_instance(instance_id: instance[:instance_id])
    instance
  end

  new_instances.each do |instance|
    wait_for_instance(instance[:instance_id], "online")
  end

  puts "Replacing existing instances.." if existing_instances.size > 0

  existing_instances.each do |instance|
    puts "Stopping instance #{instance[:hostname]}, id: #{instance[:instance_id]}"
    opsworks.stop_instance({instance_id: instance[:instance_id]})
    wait_for_instance(instance[:instance_id], "stopped")
    ebs_volume_ids = detach_ebs_volumes(instance[:instance_id])

    puts "Creating replacement instance"
    replacement = create_instance(stack_id, layer_id, instance[:availability_zone])
    attach_ebs_volumes(replacement[:instance_id], ebs_volume_ids)

    puts "Starting new instance, id: #{replacement[:instance_id]}"
    opsworks.start_instance(instance_id: replacement[:instance_id])
    wait_for_instance(replacement[:instance_id], "online")

    puts "Deleting old instance #{instance[:hostname]}, #{instance[:instance_id]}"
    opsworks.delete_instance(instance_id: instance[:instance_id])
  end
end

def min_master_node_count(instance_count)
  instance_count <= 2 ? 1 : (instance_count / 2 + 1)
end

def environment
  ENV["ENVIRONMENT"] || "my"
end

def stack_name
  "#{environment}-search"
end

def get_required(name)
  ENV[name] || raise("You must provide the environment variable #{name}")
end

def add_param_if_set(params, param_name, env_var)
  if ENV[env_var]
    "Setting CloudFormation param #{param_name.inspect} => #{ENV[env_var].inspect}"
    params.merge!(param_name => ENV[env_var])
  end
end

def cfm
  Aws::CloudFormation::Client.new
end

def cf_stack
  cfm.describe_stacks.stacks.detect{|stack| stack.stack_name == stack_name}
end

desc "Provisions the ElasticSearch cluster"
task :provision do
  instance_count = (ENV["INSTANCE_COUNT"] || "2").to_i
  data_volume_size = (ENV["DATA_VOLUME_SIZE"] || "100").to_i
  template = File.read("opsworks-service.template")

  parameters = {
    "SSLCertificateName" => get_required("SSL_CERTIFICATE_NAME"),
    "Route53ZoneName"    => get_required("ROUTE53_ZONE_NAME"),
    "SearchDomainName"   => get_required("SEARCH_DOMAIN_NAME"),

    "SshKeyName"         => ENV["SSH_KEY_NAME"] || "elasticsearch",
    "SearchUser"         => ENV["SEARCH_USER"] || "elasticsearch",
    "SearchPassword"     => ENV["SEARCH_PASSWORD"] || "pass",
    "InstanceDefaultOs"  => ENV["INSTANCE_DEFAULT_OS"] || "Amazon Linux 2015.03",
    "DataVolumeSize"     => data_volume_size.to_s,
    "InstanceCount"      => instance_count.to_s,
    "MinMasterNodes"     => min_master_node_count(instance_count).to_s,
    "ClusterName"        => "#{environment}-search-cluster",
    "RecipeList"         => DEFAULT_RECIPES
  }

  if ENV["PAPERTRAIL_ENDPOINT"].to_s =~ /[^\:]+:[\d]+/
    papertrail_host, papertrail_port = ENV["PAPERTRAIL_ENDPOINT"].split(":")
    puts "PaperTrail configured: host=#{papertrail_host}, port=#{papertrail_port}"

    parameters.merge!(
      "RecipeList" => [DEFAULT_RECIPES, "papertrail"].join(","),
      "PaperTrailHost" => papertrail_host,
      "PaperTrailPort" => papertrail_port
    )
  end

  add_param_if_set(parameters, "ElasticSearchVersion", "ELASTICSEARCH_VERSION")
  add_param_if_set(parameters, "ElasticSearchAWSCloudPluginVersion", "ELASTICSEARCH_AWS_PLUGIN_VERSION")

  params = parameters.inject([]) do |array, (key, value)|
    array << { parameter_key: key, parameter_value: value }
    array
  end

  unless cf_stack.nil?
    begin
      puts "Updating CloudFormation stack #{stack_name}"
      cfm.update_stack({stack_name: stack_name, template_body: template, parameters: params})
    rescue => e
      raise unless e.message =~ /No updates are to be performed/
      puts "Your CloudFormation stack is already up to date"
    end
  else
    puts "Creating CloudFormation stack #{stack_name}"
    cfm.create_stack({stack_name: stack_name, template_body: template, parameters: params})
  end

  wait_for_cf_stack_op_to_finish

  unless ENV["SKIP_INSTANCE_UPDATE"] == "true"
    stack_id = cf_query_output("StackId")
    layer_id = cf_query_output("LayerId")

    update_instances(stack_id, layer_id, instance_count)
  end
end

desc "Destroys the ElasticSearch cluster"
task :destroy do
  unless cf_stack.nil?
    puts "Destroying environment #{environment}"

    layer_id = cf_query_output("LayerId")

    get_all_instances(layer_id).each do |instance|
      puts "Stopping instance #{instance[:hostname]}"
      opsworks.stop_instance(instance_id: instance[:instance_id])
      wait_for_instance(instance[:instance_id], "stopped")

      puts "Deleting instance #{instance[:hostname]}"
      opsworks.delete_instance(instance_id: instance[:instance_id], delete_volumes: true)
      wait_for_instance(instance[:instance_id], "nonexistent")
    end

    puts "Deleting OpsWorks stack #{stack_name}"
    cfm.delete_stack({stack_name: stack_name})
  else
    puts "Environment #{environment} does not exist"
  end
end
