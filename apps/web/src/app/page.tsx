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
} from "@mui/material";

type DeployResponse = {
  runId: number;
  deploymentId?: number;
  status: string;
};

export default function Home() {
  const [image, setImage] = useState("nginx:alpine");
  const [serviceName, setServiceName] = useState("demo-service");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<DeployResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

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

  return (
    <>
      <CssBaseline />
      <Container maxWidth="sm" sx={{ mt: 8 }}>
        <Typography variant="h4" gutterBottom>
          AWSInfraPlatform – ECS Service Deploy (MVP)
        </Typography>

        <Box component="form" noValidate sx={{ mt: 2, display: "flex", flexDirection: "column", gap: 2 }}>
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
      </Container>
    </>
  );
}
