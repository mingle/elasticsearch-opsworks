# ElasticSearch OpsWorks

This is a fork of [ThoughtWorks Studios - Elasticsearch OpsWorks repo](https://github.com/ThoughtWorksStudios/elasticsearch-opsworks).

## Before deployment

Please setup the following dependencies in your AWS region:

* SSL certificate
* An SSH key pair, defaults to a keypair named "elasticsearch" if not specified
* A domain name for accessing the elasticsearch cluster
* A Route53 zone for the domain
* The default `aws-opsworks-service-role` and `aws-opsworks-ec2-role` need to exist before provisioning. OpsWorks should automatically create these roles when you add your first stack through the OpsWorks console. See http://docs.aws.amazon.com/opsworks/latest/userguide/gettingstarted-simple-stack.html and http://docs.aws.amazon.com/opsworks/latest/userguide/opsworks-security-appsrole.html for details.

## Setup environment

* Clone this repository
* Install jruby 1.7.16.1:
* Run bundle
* `cp env.example .env`
* Fill out values in .env to suit your deployment

## Usage

Provision the environment:

    rake provision

Open `https://<your search domain name>/_plugin/head`

Destroy the environment:

    rake destroy


## Infrastructure details

    Route53 --> ELB --> EC2 attached to EBS volumes

* Index will be stored on EBS volumes, mounted at `/mnt/elasticsearch-data`
* One master node by default, 2-node cluster by default
* Load balanced by an ELB
* Listens on HTTPS only, configured with basic auth challenge
* EC2 instance type defaults to `c3.large`
