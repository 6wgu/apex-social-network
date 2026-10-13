import "./globals.css";
import Navbar from "@/components/Navbar";

export const metadata = {
  title: "Social Network",
  description: "A Facebook-like social network",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>
        <Navbar />
        <main className="container">{children}</main>
      </body>
    </html>
  );
}
