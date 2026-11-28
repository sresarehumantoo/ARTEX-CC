---
title: Syslog, CEF and LEEF
description: Feed ARTEX alerts to QRadar, ArcSight and other syslog-based SIEMs with Vector or Fluent Bit.
sidebar:
  order: 18
---

:::caution[Draft]
Illustrative configuration against the draft `artex.audit/v1` schema. The audit stream has not shipped. See the [overview](/docs/integrations/).
:::

ARTEX has no native syslog output; shippers already do this well. Send alerts at least. Sending every egress record over syslog works but is heavy for a format with no structure beyond key=value pairs.

## CEF over syslog with Vector

The transform builds an RFC 5424 line with a CEF body for each ARTEX alert, and the socket sink sends it over TCP:

```yaml
transforms:
  artex_cef:
    type: remap
    inputs: [artex_alerts]
    source: |
      sev = if ."severity" == "WARN" { "6" } else if ."severity" == "ERROR" { "9" } else { "3" }
      cef = "CEF:0|ARTEX-CC|artex-cc|v1|" + (string(."security_rule.name") ?? "artex.alert") + "|ARTEX egress alert|" + sev +
        "|rt=" + (string(."time") ?? "") +
        " dhost=" + (string(."server.address") ?? "") +
        " suser=" + (string(."artex.identity") ?? "") +
        " cs1Label=task cs1=" + (string(."artex.task_id") ?? "")
      # RFC 5424 header, facility local0 (16), severity informational.
      . = {"message": "<134>1 " + (string(."time") ?? "-") + " artex-host artex-cc - artex-alert - " + cef}

sinks:
  legacy_siem:
    type: socket
    inputs: [artex_cef]
    mode: tcp
    address: "${SIEM_SYSLOG_ADDR}"
    framing:
      method: newline_delimited
    encoding:
      codec: text
```

Use with the [Vector base config](/docs/integrations/vector/). A sample output line:

```
<134>1 2026-11-28T15:04:09.000000001Z artex-host artex-cc - artex-alert - CEF:0|ARTEX-CC|artex-cc|v1|scope.out|ARTEX egress alert|6|rt=2026-11-28T15:04:09.000000001Z dhost=paste.example.net suser=chat/7f3a cs1Label=task cs1=t-17
```

Escape `|` in header values and `=` in extension values if your data can contain them; the sample transform does not. For TLS, put the socket sink behind stunnel or use a syslog relay; prefer TCP or TLS over UDP, which drops records silently.

## LEEF for QRadar

LEEF 2.0 has the header `LEEF:2.0|Vendor|Product|Version|EventID|<delimiter>|` followed by attributes. The same transform can produce it by changing the header and joining attributes with a tab. QRadar can also take ARTEX's JSON directly through its HTTP Receiver log source, which avoids LEEF entirely.

## Syslog with Fluent Bit

Fluent Bit's `syslog` output sends RFC 5424 over TLS; see [Fluent Bit](/docs/integrations/fluent-bit/). Its message is the field named in `syslog_message_key`, so add a filter that builds a CEF string first if the SIEM expects CEF.

## Microsoft Sentinel via CEF

Sentinel's CEF connector needs an Azure Monitor Agent forwarder running rsyslog or syslog-ng, and writes to `CommonSecurityLog`. The JSON route on the [Microsoft Sentinel](/docs/integrations/microsoft-sentinel/) page keeps more fields.

## Validation

- Vector: `vector validate` 0.62.0 passed, and the CEF transform was run against a sample alert with a console sink, producing exactly the line above.
- Fluent Bit syslog output: `fluent-bit --dry-run` 5.2.1 passed.
- The LEEF description and the CEF key choices (`dhost`, `suser`, `cs1`) were written from vendor documentation only. The individual CEF extension key definitions were not checked against the ArcSight dictionary, and nothing was sent to a SIEM.
