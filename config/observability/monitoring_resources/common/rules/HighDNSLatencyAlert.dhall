let K8s =
      https://raw.githubusercontent.com/dhall-lang/dhall-kubernetes/v6.0.0/package.dhall
        sha256:532e110f424ea8a9f960a13b2ca54779ddcac5d5aa531f86d82f41f8f18d7ef1

let TimeUnit = ../../dhall/TimeUnit/package.dhall

let Duration = ../../dhall/Duration/package.dhall

let AlertSeverity = ../../dhall/AlertSeverity/package.dhall

let PrometheusOperator =
      ( https://raw.githubusercontent.com/coralogix/dhall-prometheus-operator/v8.0.0/package.dhall
          sha256:ebc5f0c5f57d410412c2b7cbb64d0883be648eafc094f0c3e10dba4e6bd46ed4
      ).v1

in  PrometheusOperator.Rule::{
    , alert = Some "HighDNSLatencyAlert"
    , expr = K8s.IntOrString.String "vector(1)"
    , for = Some (Duration.show { amount = 60, unit = TimeUnit.Type.Minutes })
    , labels = Some
        (toMap { severity = AlertSeverity.show AlertSeverity.Type.Warning })
    , annotations = Some
        ( toMap
            { summary = "High DNS Latency Rate Alert"
            , description = "Description"
            , runbook_url =
                "https://github.com/Kuadrant/kcp-glbc/blob/main/docs/observability/runbooks/HighDNSLatencyAlert.adoc"
            }
        )
    }
