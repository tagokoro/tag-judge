// The ALB health check for the frontend target group. It answers without
// calling the Go API, so an API outage does not mark the frontend unhealthy.
export const dynamic = "force-dynamic";

export function GET() {
  return new Response(null, { status: 204 });
}
