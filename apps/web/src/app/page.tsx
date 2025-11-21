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

  return (
    <>
      <CssBaseline />
      <Container maxWidth="sm" sx={{ mt: 8 }}>
        <Typography variant="h4" gutterBottom>
          AWSInfraPlatform – ECS Service Deploy (MVP)
        </Typography>

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
                No connections loaded yet. Click &quot;Load AWS Connections&quot; to
                fetch.
              </Typography>
            )}
        </Box>
      </Container>
    </>
  );
}
