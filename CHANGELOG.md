# Changelog

## 0.12.0 (2026-11-12)

### Breaking changes

The CubePath API now names a baremetal server only by its uuid: server numbers are no longer
accepted or returned anywhere. Every attribute that held a server number holds the uuid now.

- `cubepath_baremetal`: `id` is the server uuid (it was the server number) and `uuid` carries
  the same value. Import takes the uuid only. A state written by an earlier version keeps the
  number in `id` and cannot be read any more: the plan fails with "Baremetal ID is a server
  number". For each such server run

  ```shell
  terraform state rm cubepath_baremetal.<name>
  terraform import cubepath_baremetal.<name> <uuid>
  ```

  with the uuid shown in the CubePath dashboard or by `cubecli baremetal list`. Nothing is
  changed on the server. As after any import of a baremetal server, the next plan shows an
  in-place update of that resource: it only records the password, which import cannot read,
  in the state.
- `cubepath_baremetal`: a server that is no longer found in any project is a read error instead
  of a removal from the state, so the next plan never deploys (and bills) a second server. If
  it was cancelled on purpose, remove it with `terraform state rm`.
- `cubepath_floating_ip`: `assign_baremetal_id` is a string with the server uuid (it was a
  number). The state is upgraded on its own (schema version 1), but the provider cannot turn the
  stored number into the uuid, because the API no longer knows numbers: the number is kept as
  text, and the next refresh replaces it with the uuid the API reports for the address. Write
  the uuid in the configuration (or `cubepath_baremetal.<name>.id`) and the plan shows no
  change: the address is not moved. A number in the configuration fails validation.
- `cubepath_network_route.next_hop_target`, `cubepath_network_bgp_peer.peer_target`,
  `cubepath_lb_target.target_id` and `cubepath_alert_rule.target_id` take the server uuid when
  the type is `baremetal`, and a number fails validation before any request. The refresh
  replaces the number kept by an earlier version with the uuid the API returns, so a
  configuration that names the uuid (or `cubepath_baremetal.<name>.id`) shows no change and
  nothing is replaced. Write the uuid in lower case, as the API returns it: these attributes
  force a new resource when they change.
