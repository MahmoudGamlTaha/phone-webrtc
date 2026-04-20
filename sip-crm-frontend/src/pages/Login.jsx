import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input"; // I will create this component next

export default function LoginPage() {
  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <div className="w-full max-w-md p-8 space-y-6 bg-card text-card-foreground rounded-lg shadow-lg">
        <div className="text-center">
          <h1 className="text-3xl font-bold">Login</h1>
          <p className="text-muted-foreground">Enter your credentials to access your account</p>
        </div>
        <div className="space-y-4">
          <div className="space-y-2">
            <label htmlFor="username">Username</label>
            <Input id="username" placeholder="Your username" />
          </div>
          <div className="space-y-2">
            <label htmlFor="password">Password</label>
            <Input id="password" type="password" placeholder="Your password" />
          </div>
        </div>
        <Button className="w-full">Login</Button>
      </div>
    </div>
  );
}
