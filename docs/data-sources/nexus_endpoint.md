---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "temporalcloud_nexus_endpoint Data Source - terraform-provider-temporalcloud"
subcategory: ""
description: |-
  Fetches details about a Nexus Endpoint.
---

# temporalcloud_nexus_endpoint (Data Source)

Fetches details about a Nexus Endpoint.

## Example Usage

```terraform
terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

resource "temporalcloud_namespace" "target_namespace" {
  name           = "terraform-target-namespace"
  regions        = ["aws-us-west-2"]
  api_key_auth   = true
  retention_days = 14
  timeouts {
    create = "10m"
    delete = "10m"
  }
}

resource "temporalcloud_namespace" "caller_namespace" {
  name           = "terraform-caller-namespace"
  regions        = ["aws-us-east-1"]
  api_key_auth   = true
  retention_days = 14
  timeouts {
    create = "10m"
    delete = "10m"
  }
}

resource "temporalcloud_namespace" "caller_namespace_2" {
  name           = "terraform-caller-namespace-2"
  regions        = ["gcp-us-central1"]
  api_key_auth   = true
  retention_days = 14
  timeouts {
    create = "10m"
    delete = "10m"
  }
}

resource "temporalcloud_nexus_endpoint" "nexus_endpoint" {
  name        = "terraform-nexus-endpoint"
  description = <<-EOT
    Service Name:
      my-hello-service
    Operation Names:
      echo
      say-hello

    Input / Output arguments are in the following repository:
    https://github.com/temporalio/samples-go/blob/main/nexus/service/api.go
  EOT
  worker_target = {
    namespace_id = temporalcloud_namespace.target_namespace.id
    task_queue   = "terraform-task-queue"
  }
  allowed_caller_namespaces = [
    temporalcloud_namespace.caller_namespace.id,
    temporalcloud_namespace.caller_namespace_2.id,
  ]
}

data "temporalcloud_nexus_endpoint" "example" {
  id = temporalcloud_nexus_endpoint.nexus_endpoint
}

output "nexus_endpoint" {
  value = data.temporalcloud_nexus_endpoint.example
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) The unique identifier of the Nexus Endpoint.

### Optional

- `description` (String, Sensitive) The description of the Nexus Endpoint.

### Read-Only

- `allowed_caller_namespaces` (Set of String) Namespace Id(s) that are allowed to call this Endpoint.
- `created_at` (String) The creation time of the Nexus Endpoint.
- `name` (String) The name of the endpoint. Unique within an account and match `^[a-zA-Z][a-zA-Z0-9\-]*[a-zA-Z0-9]$`
- `state` (String) The current state of the Nexus Endpoint.
- `updated_at` (String) The last update time of the Nexus Endpoint.
- `worker_target` (Attributes) The target spec for routing nexus requests to a specific cloud namespace worker. (see [below for nested schema](#nestedatt--worker_target))

<a id="nestedatt--worker_target"></a>
### Nested Schema for `worker_target`

Read-Only:

- `namespace_id` (String) The target cloud namespace to route requests to. Namespace is in same account as the endpoint.
- `task_queue` (String) The task queue on the cloud namespace to route requests to.
