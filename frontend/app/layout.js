import "./globals.css";
import "./apex.css";
import AuthProvider, { AuthGate } from "@/components/AuthProvider";
import Footer from "@/components/Footer";
import Navbar from "@/components/Navbar";
import RacingCursor from "@/components/RacingCursor";
import Toaster from "@/components/Toaster";
import RealtimeProvider from "@/components/RealtimeProvider";
import UnreadMessagesProvider from "@/components/UnreadMessages";

export const metadata = {
  title: "APEX | Connect at Full Speed.",
  description: "APEX is a premium social network for the people and conversations that matter.",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>
        <AuthProvider>
          <RealtimeProvider>
            <UnreadMessagesProvider>
              <Navbar />
              <main className="container">
                <AuthGate>{children}</AuthGate>
              </main>
              <Footer />
              <Toaster />
            </UnreadMessagesProvider>
          </RealtimeProvider>
        </AuthProvider>
        <RacingCursor />
      </body>
    </html>
  );
}
