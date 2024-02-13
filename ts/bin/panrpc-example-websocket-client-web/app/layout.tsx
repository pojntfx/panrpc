import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "panrpc WebSocket Client Example (Web)",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
