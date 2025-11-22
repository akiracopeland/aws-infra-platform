"use client";

import { useState } from "react";
import {
  Box,
  Button,
  Container,
  CssBaseline,
  TextField,
  Typography,
  Alert,
  Card,
  CardContent,
  Stack,
  MenuItem,
} from "@mui/material";

type DeployResponse = {
  runId: number;
  deploymentId?: number;
  status: string;
};

type AwsConnection = {
  id: number;
  orgId: number;
  accountId: string;
  roleArn: string;
  externalId: string;
  region: string;
  nickname?: string;
};

type RunStatusResponse = {
  id: number;
  deploymentId: number;
  action: string;
  status: string;
  startedAt?: string;
  finishedAt?: string;
  summary?: string;
};

type DeploymentSummary = {
  id: number;
  blueprintKey: string;
  version: string;
  environmentId: number;
  status: string;
  createdAt: string;
  lastRunId?: number;
  lastRunStatus?: string;
  lastRunStartedAt?: string;
  lastRunFinishedAt?: string;
  // Raw terraform outputs JSON (terraform output -json)
  outputsJson?: any;
};

export default function Home() {
  const [image, setImage] = useState("nginx:alpine");
  const [serviceName, setServiceName] = useState("demo-service");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<DeployResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [connections, setConnections] = useState<AwsConnection[]>([]);
  const [connectionsError, setConnectionsError] = useState<string | null>(null);
  const [loadingConnections, setLoadingConnections] = useState(false);
  const [selectedConnectionId, setSelectedConnectionId] = useState<number | null>(null);

  // Add-connection form state
  const [newRoleArn, setNewRoleArn] = useState("");
  const [newExternalId, setNewExternalId] = useState("");
  const [newRegion, setNewRegion] = useState("ap-northeast-1");
  const [newNickname, setNewNickname] = useState("");
  const [creatingConnection, setCreatingConnection] = useState(false);
  const [createConnectionError, setCreateConnectionError] = useState<string | null>(null);
  const [createConnectionSuccess, setCreateConnectionSuccess] = useState<string | null>(null);

  // Run status UI state
  const [runIdQuery, setRunIdQuery] = useState("");
  const [runStatus, setRunStatus] = useState<RunStatusResponse | null>(null);
  const [runStatusError, setRunStatusError] = useState<string | null>(null);
  const [runStatusLoading, setRunStatusLoading] = useState(false);

  // Deployments list state
  const [deployments, setDeployments] = useState<DeploymentSummary[]>([]);
  const [deploymentsError, setDeploymentsError] = useState<string | null>(null);
  const [loadingDeployments, setLoadingDeployments] = useState(false);

  async function handleDeploy() {
    setLoading(true);
    setError(null);
    setResult(null);

    const conn = connections.find((c) => c.id === selectedConnectionId);
    if (!conn) {
      setError("Please select an AWS connection before deploying.");
      setLoading(false);
      return;
    }

    try {
      const res = await fetch(
        `${process.env.NEXT_PUBLIC_API_BASE}/v1/deployments`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            blueprintKey: "ecs-service",
            version: "1.0.0",
            environmentId: 1,
            action: "plan",
            inputs: {
              serviceName,
              image,
              cpu: 256,
              memory: 512,
            },
            aws: {
              roleArn: conn.roleArn,
              externalId: conn.externalId,
              region: conn.region,
            },
          }),
        }
      );

      if (!res.ok) {
        const text = await res.text();
        throw new Error(`API error ${res.status}: ${text}`);
      }

      const json = (await res.json()) as DeployResponse;
      setResult(json);

      // Refresh deployments list after a new deployment
      void loadDeployments();
    } catch (err: any) {
      setError(err.message || "Unknown error");
    } finally {
      setLoading(false);
    }
  }

  async function handleApply() {
    setLoading(true);
    setError(null);
    setResult(null);

    const conn = connections.find((c) => c.id === selectedConnectionId);
    if (!conn) {
      setError("Please select an AWS connection before deploying.");
      setLoading(false);
      return;
    }

    try {
      const res = await fetch(
        `${process.env.NEXT_PUBLIC_API_BASE}/v1/deployments`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            blueprintKey: "ecs-service",
            version: "1.0.0",
            environmentId: 1,
            action: "apply",
            inputs: {
              serviceName,
              image,
              cpu: 256,
              memory: 512,
            },
            aws: {
              roleArn: conn.roleArn,
              externalId: conn.externalId,
              region: conn.region,
            },
          }),
        }
      );

      if (!res.ok) {
        const text = await res.text();
        throw new Error(`API error ${res.status}: ${text}`);
      }

      const json = (await res.json()) as DeployResponse;
      setResult(json);

      // Refresh deployments list after a new deployment
      void loadDeployments();
    } catch (err: any) {
      setError(err.message || "Unknown error");
    } finally {
      setLoading(false);
    }
  }

  async function handleDestroy(deploymentId: number) {
    setError(null);
    setResult(null);

    const conn = connections.find((c) => c.id === selectedConnectionId);
    if (!conn) {
      setError("Please select an AWS connection before destroying.");
      return;
    }

    try {
      const res = await fetch(
        `${process.env.NEXT_PUBLIC_API_BASE}/v1/deployments/${deploymentId}/destroy`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            aws: {
              roleArn: conn.roleArn,
              externalId: conn.externalId,
              region: conn.region,
            },
          }),
        }
      );

      if (!res.ok) {
        const text = await res.text();
        throw new Error(`API error ${res.status}: ${text}`);
      }

      const json = (await res.json()) as DeployResponse;
      setResult(json);

      // Refresh list
      void loadDeployments();
    } catch (err: any) {
        setError(err.message || "Failed to queue destroy");
    }
  }


  async function loadConnections() {
    setLoadingConnections(true);
    setConnectionsError(null);

    try {
      const res = await fetch(
        `${process.env.NEXT_PUBLIC_API_BASE}/v1/connections/aws`
      );

      if (!res.ok) {
        const text = await res.text();
        throw new Error(`API error ${res.status}: ${text}`);
      }

      const json = (await res.json()) as AwsConnection[];
      setConnections(json);

      if (json.length > 0 && selectedConnectionId === null) {
        setSelectedConnectionId(json[0].id);
      }
    } catch (err: any) {
      setConnectionsError(err.message || "Failed to load connections");
    } finally {
      setLoadingConnections(false);
    }
  }

  async function handleAddConnection() {
    setCreatingConnection(true);
    setCreateConnectionError(null);
    setCreateConnectionSuccess(null);

    try {
      const res = await fetch(
        `${process.env.NEXT_PUBLIC_API_BASE}/v1/connections/aws`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            roleArn: newRoleArn,
            externalId: newExternalId,
            region: newRegion,
            nickname: newNickname || undefined,
          }),
        }
      );

      if (!res.ok) {
        const text = await res.text();
        throw new Error(`API error ${res.status}: ${text}`);
      }

      const created = (await res.json()) as AwsConnection;

      setConnections((prev) => [created, ...prev]);
      setSelectedConnectionId(created.id);
      setCreateConnectionSuccess("AWS connection created successfully");

      setNewRoleArn("");
      setNewExternalId("");
      setNewNickname("");
    } catch (err: any) {
      setCreateConnectionError(err.message || "Failed to create connection");
    } finally {
      setCreatingConnection(false);
    }
  }

  async function handleCheckRunStatus() {
    setRunStatusLoading(true);
    setRunStatusError(null);
    setRunStatus(null);

    const id = runIdQuery.trim();
    if (!id) {
      setRunStatusError("Please enter a Run ID.");
      setRunStatusLoading(false);
      return;
    }

    try {
      const res = await fetch(
        `${process.env.NEXT_PUBLIC_API_BASE}/v1/runs/${encodeURIComponent(id)}`
      );

      if (!res.ok) {
        const text = await res.text();
        throw new Error(`API error ${res.status}: ${text}`);
      }

      const json = (await res.json()) as RunStatusResponse;
      setRunStatus(json);
    } catch (err: any) {
      setRunStatusError(err.message || "Failed to fetch run status");
    } finally {
      setRunStatusLoading(false);
    }
  }

  async function loadDeployments() {
    setLoadingDeployments(true);
    setDeploymentsError(null);

    try {
      const res = await fetch(
        `${process.env.NEXT_PUBLIC_API_BASE}/v1/deployments`
      );

      if (!res.ok) {
        const text = await res.text();
        throw new Error(`API error ${res.status}: ${text}`);
      }

      const json = (await res.json()) as DeploymentSummary[];
      setDeployments(json);
    } catch (err: any) {
      setDeploymentsError(err.message || "Failed to load deployments");
    } finally {
      setLoadingDeployments(false);
    }
  }

  return (
    <>
      <CssBaseline />
      <Container maxWidth="sm" sx={{ mt: 8 }}>
        <Typography variant="h4" gutterBottom>
          AWSInfraPlatform – ECS Service Deploy (MVP)
        </Typography>

        {/* Deploy form */}
        <Box
          component="form"
          noValidate
          sx={{ mt: 2, display: "flex", flexDirection: "column", gap: 2 }}
        >
          <TextField
            label="Service Name"
            value={serviceName}
            onChange={(e) => setServiceName(e.target.value)}
            fullWidth
          />
          <TextField
            label="Container Image"
            value={image}
            onChange={(e) => setImage(e.target.value)}
            fullWidth
          />

          <Box sx={{ display: "flex", gap: 2 }}>
            <Button
              variant="contained"
              onClick={handleDeploy}
              disabled={loading}
            >
              {loading ? "Deploying..." : "Plan ECS Service"}
            </Button>

            <Button
              variant="outlined"
              color="secondary"
              onClick={handleApply}
              disabled={loading}
            >
              {loading ? "Applying..." : "Apply ECS Service"}
            </Button>
          </Box>
        </Box>

        {error && (
          <Alert severity="error" sx={{ mt: 3 }}>
            {error}
          </Alert>
        )}

        {result && (
          <Alert severity="success" sx={{ mt: 3 }}>
            <div>Deployment queued!</div>
            <div>Deployment ID: {result.deploymentId}</div>
            <div>Run ID: {result.runId}</div>
            <div>Status: {result.status}</div>
          </Alert>
        )}

        {/* AWS Connections section */}
        <Box sx={{ mt: 6 }}>
          <Typography variant="h5" gutterBottom>
            AWS Connections
          </Typography>

          {/* Add AWS Connection form */}
          <Box
            sx={{
              mt: 2,
              mb: 3,
              display: "flex",
              flexDirection: "column",
              gap: 2,
            }}
          >
            <TextField
              label="Role ARN"
              placeholder="arn:aws:iam::882573817622:role/aip-target-deploy"
              value={newRoleArn}
              onChange={(e) => setNewRoleArn(e.target.value)}
              fullWidth
            />
            <TextField
              label="External ID"
              placeholder="aip-dev-external-id"
              value={newExternalId}
              onChange={(e) => setNewExternalId(e.target.value)}
              fullWidth
            />
            <TextField
              label="Region"
              value={newRegion}
              onChange={(e) => setNewRegion(e.target.value)}
              fullWidth
            />
            <TextField
              label="Nickname (optional)"
              placeholder="dev-target"
              value={newNickname}
              onChange={(e) => setNewNickname(e.target.value)}
              fullWidth
            />

            <Button
              variant="contained"
              onClick={handleAddConnection}
              disabled={creatingConnection}
            >
              {creatingConnection ? "Creating..." : "Add AWS Connection"}
            </Button>

            {createConnectionError && (
              <Alert severity="error">{createConnectionError}</Alert>
            )}
            {createConnectionSuccess && (
              <Alert severity="success">{createConnectionSuccess}</Alert>
            )}
          </Box>

          {/* Select connection */}
          <Box sx={{ mb: 2 }}>
            <TextField
              select
              fullWidth
              label="Selected AWS Connection"
              value={selectedConnectionId ?? ""}
              onChange={(e) =>
                setSelectedConnectionId(
                  e.target.value === "" ? null : Number(e.target.value)
                )
              }
              helperText="This connection will be used for deployments"
            >
              {connections.map((conn) => (
                <MenuItem key={conn.id} value={conn.id}>
                  {conn.nickname || "(no nickname)"} — {conn.accountId} (
                  {conn.region})
                </MenuItem>
              ))}
            </TextField>
          </Box>

          {/* Load connections + list */}
          <Button
            variant="outlined"
            onClick={loadConnections}
            disabled={loadingConnections}
          >
            {loadingConnections ? "Loading..." : "Load AWS Connections"}
          </Button>

          {connectionsError && (
            <Alert severity="error" sx={{ mt: 2 }}>
              {connectionsError}
            </Alert>
          )}

          {connections.length > 0 && (
            <Stack spacing={2} sx={{ mt: 2 }}>
              {connections.map((conn) => (
                <Card key={conn.id}>
                  <CardContent>
                    <Typography variant="subtitle1">
                      {conn.nickname || "(no nickname)"}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      Account: {conn.accountId}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      Region: {conn.region}
                    </Typography>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mt: 1, wordBreak: "break-all" }}
                    >
                      Role ARN: {conn.roleArn}
                    </Typography>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mt: 1 }}
                    >
                      External ID: {conn.externalId}
                    </Typography>
                  </CardContent>
                </Card>
              ))}
            </Stack>
          )}

          {connections.length === 0 &&
            !connectionsError &&
            !loadingConnections && (
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ mt: 2 }}
              >
                No connections loaded yet. Create one above or click &quot;Load
                AWS Connections&quot; to fetch existing ones.
              </Typography>
            )}
        </Box>

        {/* Run Status section */}
        <Box sx={{ mt: 6 }}>
          <Typography variant="h5" gutterBottom>
            Run Status
          </Typography>

          <Box
            sx={{
              mt: 2,
              display: "flex",
              flexDirection: "column",
              gap: 2,
            }}
          >
            <TextField
              label="Run ID"
              value={runIdQuery}
              onChange={(e) => setRunIdQuery(e.target.value)}
              fullWidth
            />
            <Button
              variant="outlined"
              onClick={handleCheckRunStatus}
              disabled={runStatusLoading}
            >
              {runStatusLoading ? "Checking..." : "Check Run Status"}
            </Button>

            {runStatusError && (
              <Alert severity="error">{runStatusError}</Alert>
            )}

            {runStatus && (
              <Card>
                <CardContent>
                  <Typography variant="subtitle1">
                    Run #{runStatus.id} – {runStatus.status}
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    Deployment ID: {runStatus.deploymentId}
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    Action: {runStatus.action}
                  </Typography>
                  {runStatus.startedAt && (
                    <Typography variant="body2" color="text.secondary">
                      Started: {runStatus.startedAt}
                    </Typography>
                  )}
                  {runStatus.finishedAt && (
                    <Typography variant="body2" color="text.secondary">
                      Finished: {runStatus.finishedAt}
                    </Typography>
                  )}
                  {runStatus.summary && (
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mt: 1, whiteSpace: "pre-wrap" }}
                    >
                      Summary: {runStatus.summary}
                    </Typography>
                  )}
                </CardContent>
              </Card>
            )}
          </Box>
        </Box>

        {/* Deployments history section */}
        <Box sx={{ mt: 6, mb: 8 }}>
          <Typography variant="h5" gutterBottom>
            Deployments
          </Typography>

          <Button
            variant="outlined"
            onClick={loadDeployments}
            disabled={loadingDeployments}
          >
            {loadingDeployments ? "Loading..." : "Load Deployments"}
          </Button>

          {deploymentsError && (
            <Alert severity="error" sx={{ mt: 2 }}>
              {deploymentsError}
            </Alert>
          )}

          {deployments.length > 0 && (
            <Stack spacing={2} sx={{ mt: 2 }}>
              {deployments.map((dep) => {
                const outputs = dep.outputsJson as any | undefined;

                const serviceId =
                  outputs?.service_id?.value ??
                  outputs?.summary?.value?.serviceId;
                const clusterId =
                  outputs?.cluster_id?.value ??
                  outputs?.summary?.value?.clusterId;
                const logGroup =
                  outputs?.log_group?.value ??
                  outputs?.summary?.value?.logGroup;

                return (
                  <Card key={dep.id}>
                    <CardContent>
                      <Typography variant="subtitle1">
                        Deployment #{dep.id} – {dep.blueprintKey} @ {dep.version}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        Environment ID: {dep.environmentId}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        Deployment status: {dep.status}
                      </Typography>

                      <Box sx={{ mt: 1 }}>
                        <Button
                          variant="outlined"
                          color="error"
                          size="small"
                          onClick={() => {
                            if (
                              window.confirm(
                                `Destroy deployment #${dep.id}? This will run 'terraform destroy' for its resources.`
                              )
                            ) {
                              void handleDestroy(dep.id);
                            }
                          }}
                          disabled={dep.status === "destroying" || dep.status === "destroyed"}
                        >
                          Destroy deployment
                        </Button>
                      </Box>

                      {dep.lastRunId && (
                        <Typography variant="body2" color="text.secondary">
                          Last run: #{dep.lastRunId} (
                          {dep.lastRunStatus ?? "unknown"})
                        </Typography>
                      )}
                      {dep.lastRunStartedAt && (
                        <Typography variant="body2" color="text.secondary">
                          Last run started: {dep.lastRunStartedAt}
                        </Typography>
                      )}
                      {dep.lastRunFinishedAt && (
                        <Typography variant="body2" color="text.secondary">
                          Last run finished: {dep.lastRunFinishedAt}
                        </Typography>
                      )}

                      {serviceId && (
                        <Typography variant="body2" color="text.secondary">
                          Service ARN: {serviceId}
                        </Typography>
                      )}
                      {clusterId && (
                        <Typography variant="body2" color="text.secondary">
                          Cluster ARN: {clusterId}
                        </Typography>
                      )}
                      {logGroup && (
                        <Typography variant="body2" color="text.secondary">
                          Log group: {logGroup}
                        </Typography>
                      )}

                      <Typography variant="body2" color="text.secondary">
                        Created: {dep.createdAt}
                      </Typography>
                    </CardContent>
                  </Card>
                );
              })}
            </Stack>
          )}

          {deployments.length === 0 &&
            !deploymentsError &&
            !loadingDeployments && (
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ mt: 2 }}
              >
                No deployments loaded yet. Click &quot;Load Deployments&quot; to
                fetch history.
              </Typography>
            )}
        </Box>
      </Container>
    </>
  );
}
