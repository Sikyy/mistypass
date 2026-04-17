import { FormEvent, useState } from "react"
import { ArrowRightIcon, Building2Icon, LockKeyholeIcon, RadioTowerIcon } from "lucide-react"
import { useNavigate } from "react-router-dom"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuth } from "@/context/auth-context"
import { login } from "@/lib/api"

export function LoginPage() {
  const navigate = useNavigate()
  const { setAuthenticatedSession } = useAuth()
  const showDevTestAccounts = import.meta.env.DEV && import.meta.env.VITE_SHOW_TEST_ACCOUNTS !== "false"
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [submitting, setSubmitting] = useState(false)

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setError("")

    try {
      const response = await login(email, password)
      setAuthenticatedSession(response.access_token, response.refresh_token, response.user)
      navigate("/dashboard", { replace: true })
    } catch (err) {
      const message = err instanceof Error ? err.message : "登录失败"
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_15%_5%,rgba(20,184,166,0.16),transparent_35%),radial-gradient(circle_at_80%_100%,rgba(14,116,144,0.2),transparent_40%)]">
      <div className="grid min-h-screen lg:grid-cols-2">
        <section className="hidden border-r bg-gradient-to-br from-teal-500/15 via-background to-cyan-500/10 p-10 lg:flex lg:flex-col lg:justify-between">
          <div className="inline-flex w-fit items-center gap-2 rounded-full border bg-background/70 px-3 py-1 text-xs text-muted-foreground backdrop-blur">
            <RadioTowerIcon className="size-3.5" />
            MistyPass 企业与空间访问平台
          </div>

          <div className="space-y-6">
            <h1 className="max-w-md text-4xl font-semibold">企业、楼宇与门禁发放的一体化工作台</h1>
            <p className="max-w-lg text-sm text-muted-foreground">
              同时支持平台管理、楼宇值守和企业自营场景，强调目录同步、凭证发放与审计追踪。
            </p>
            <div className="grid max-w-md gap-2">
              <div className="flex items-center gap-2 rounded-lg border bg-background/80 px-3 py-2 text-sm">
                <Building2Icon className="size-4 text-primary" />
                组织级数据隔离与角色工作台
              </div>
              <div className="flex items-center gap-2 rounded-lg border bg-background/80 px-3 py-2 text-sm">
                <LockKeyholeIcon className="size-4 text-primary" />
                员工导入、SSO、发放与事件记录
              </div>
            </div>
          </div>

          <p className="mp-kpi-note">MistyPass 管理后台 · MVP</p>
        </section>

        <section className="flex items-center justify-center p-4 sm:p-8">
          <Card className="w-full max-w-md">
            <CardHeader className="space-y-2">
              <CardTitle className="text-2xl">登录</CardTitle>
              <CardDescription>进入企业、空间、发放与安全处置工作台。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <form onSubmit={onSubmit} className="space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="email">邮箱</Label>
                  <Input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(event) => setEmail(event.target.value)}
                    placeholder="name@company.com"
                    autoComplete="email"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="password">密码</Label>
                  <Input
                    id="password"
                    type="password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    placeholder="********"
                    autoComplete="current-password"
                  />
                </div>

                {error ? (
                  <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                    {error}
                  </div>
                ) : null}

                <Button type="submit" className="w-full" disabled={submitting}>
                  {submitting ? "登录中..." : "登录"}
                  <ArrowRightIcon className="ml-1.5 size-4" />
                </Button>
              </form>

              {showDevTestAccounts ? (
                <p className="mp-kpi-note">
                  开发环境测试账号示例：`superadmin@mistypass.local`、`tenant.admin@sudirman.co`、`building.admin.sudirman@mistypass.local`，密码均为 `admin123`。
                </p>
              ) : null}
            </CardContent>
          </Card>
        </section>
      </div>
    </div>
  )
}
