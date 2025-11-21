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
  region: string;
  nickname?: string;
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

  // Add-connection form state
  const [newRoleArn, setNewRoleArn] = useState("");
  const [newExternalId, setNewExternalId] = useState("");
  const [newRegion, setNewRegion] = useState("ap-northeast-1");
  const [newNickname, setNewNickname] = useState("");
  const [creatingConnection, setCreatingConnection] = useState(false);
  const [createConnectionError, setCreateConnectionError] = useState<string | null>(null);
  const [createConnectionSuccess, setCreateConnectionSuccess] = useState<string | null>(null);

  async function handleDeploy() {
    setLoading(true);
    setError(null);
    setResult(null);

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
            inputs: {
              serviceName,
              image,
              cpu: 256,
              memory: 512,
            },
            aws: {
              // TODO: later, pick this from a selected connection instead of hardcoding
              roleArn: "arn:aws:iam::123456789012:role/YourTargetRole",
              externalId: "example-external-id",
              region: "ap-northeast-1",
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
    } catch (err: any) {
      setError(err.message || "Unknown error");
    } finally {
      setLoading(false);
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

      // Prepend new connection to the list
      setConnections((prev) => [created, ...prev]);
      setCreateConnectionSuccess("AWS connection created successfully");

      // Clear form fields (keep region default)
      setNewRoleArn("");
      setNewExternalId("");
      setNewNickname("");
    } catch (err: any) {
      setCreateConnectionError(err.message || "Failed to create connection");
    } finally {
      setCreatingConnection(false);
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

          <Button
            variant="contained"
            onClick={handleDeploy}
            disabled={loading}
          >
            {loading ? "Deploying..." : "Deploy ECS Service (Plan)"}
          </Button>
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

          {/* List / refresh button */}
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
      </Container>
    </>
  );
}
