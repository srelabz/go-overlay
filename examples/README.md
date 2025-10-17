# Real-World Stack Examples with go-overlay

This directory contains production-ready examples of different web stacks using go-overlay to manage multiple services in a single container.

## 📚 Examples Index

### 1. [FastAPI + React + Caddy](./fastapi-react-stack/)

**Modern Python stack with REST API and React frontend**

- 🐍 Backend: FastAPI with Uvicorn
- ⚛️  Frontend: React with Vite
- 🌐 Proxy: Caddy
- 🔄 Migrations: Alembic
- 📦 Single container with 3 services

**Ideal for:**
- Python REST APIs
- Machine Learning backends
- Projects with specific Python libraries

[📖 View full documentation →](./fastapi-react-stack/README.md)

---

### 2. [Express.js + React + Caddy](./express-react-stack/)

**Full-stack Node.js with Prisma ORM**

- 🟢 Backend: Express.js
- ⚛️  Frontend: React with Vite
- 🌐 Proxy: Caddy
- 🔄 Migrations: Prisma
- 📦 Single container with 3 services

**Ideal for:**
- JavaScript/TypeScript full-stack
- Rapid development
- Node.js teams

[📖 View full documentation →](./express-react-stack/README.md)

---

### 3. [Next.js Standalone](./nextjs-standalone/)

**Next.js in standalone mode with SSR/SSG**

- ⚡ Framework: Next.js 14+
- 🎨 Server-Side Rendering
- 📄 Static Site Generation
- 🔌 Integrated API Routes
- 📦 Single container, single service

**Ideal for:**
- Critical SEO
- Integrated full-stack applications
- Static + dynamic pages
- React projects with SSR

[📖 View full documentation →](./nextjs-standalone/README.md)

---

### 4. [Bun + React + Caddy](./bun-react-stack/)

**Performance Stack with Bun runtime**

- 🚀 Backend: Bun (ultra-fast runtime)
- ⚛️  Frontend: React with Vite
- 🌐 Proxy: Caddy
- ⚡ Native TypeScript
- 📦 Single container with 2 services

**Ideal for:**
- Maximum performance (3x faster than Node.js)
- TypeScript without compilation
- Projects needing reduced infrastructure costs
- Rapid development

[📖 View full documentation →](./bun-react-stack/README.md)

---

### 5. [Production Stack](./production-stack/) ⭐

**Complete production stack with cache and database**

- 🐍 Backend: FastAPI
- ⚛️  Frontend: React with Vite
- 🌐 Proxy: Caddy
- 🗄️ Database: PostgreSQL
- 🔴 Cache: Redis
- 🔄 Migrations: Alembic
- ✅ Health Checks
- 📦 Single container with 5 services

**Ideal for:**
- Robust production applications
- Apps requiring cache
- Complex relational data
- High read performance
- User sessions
- Rate limiting

[📖 View full documentation →](./production-stack/README.md)

---

## 🎯 Which Stack to Choose?

Use this table to decide which example best fits your project:

| Need | Recommended Stack |
|------|-------------------|
| **REST API + Separate Frontend** | FastAPI + React or Express + React |
| **Python backend** | FastAPI + React |
| **JavaScript full-stack** | Express + React or Next.js |
| **Maximum performance** | Bun + React |
| **Native TypeScript** | Bun + React |
| **SEO important** | Next.js Standalone |
| **Cache needed** | Production Stack |
| **Relational database** | Production Stack or any with migrations |
| **Simple application** | FastAPI + React, Express + React, or Bun + React |
| **Complex application** | Production Stack |
| **Machine Learning** | FastAPI + React |
| **Real-time / WebSocket** | Production Stack |
| **Reduce infrastructure costs** | Bun + React (uses fewer resources) |

---

## 🚀 How to Use the Examples

Each example includes:

1. **Complete README.md** with detailed explanations
2. **services.toml** configured and ready to use
3. **Functional Dockerfile**
4. **Configuration files** (Caddyfile, etc)
5. **Code examples** for backend and frontend
6. **Step-by-step instructions** for build and deploy

### General Steps

```bash
# 1. Enter the example directory
cd examples/fastapi-react-stack/

# 2. Read the README
cat README.md

# 3. Follow the example-specific instructions
# (each stack has its particularities)
```

---

## 📦 Common Structure

All examples follow a similar structure:

```
example-name/
├── README.md          # Complete documentation
├── services.toml      # go-overlay configuration
├── Caddyfile         # Proxy configuration (if applicable)
└── (other specific config files)
```

---

## 🔧 Demonstrated Features

Each example demonstrates different go-overlay features:

### FastAPI + React
- ✅ Oneshot migrations (Alembic)
- ✅ Dependency management with `depends_on`
- ✅ Reverse proxy with Caddy
- ✅ CORS configured

### Express + React
- ✅ Oneshot migrations (Prisma)
- ✅ Node.js with TypeScript
- ✅ Prisma ORM
- ✅ Dependency management

### Bun + React
- ✅ Ultra-fast runtime (3x faster than Node.js)
- ✅ Native TypeScript without compilation
- ✅ Integrated SQLite
- ✅ Native hot reload
- ✅ Lower resource usage

### Next.js
- ✅ Standalone mode
- ✅ SSR/SSG
- ✅ Integrated API Routes
- ✅ Minimal configuration

### Production Stack
- ✅ Multiple dependencies (Redis + PostgreSQL)
- ✅ Health checks with `pre_script`
- ✅ Service validation before startup
- ✅ Distributed cache
- ✅ WebSocket support
- ✅ Graceful shutdown
- ✅ Configured timeouts

---

## 💡 Tips

1. **Start simple**: If learning, start with FastAPI + React or Express + React
2. **Go to production**: When ready, use Production Stack as a base
3. **Customize**: All examples are starting points, adapt to your needs
4. **Read the comments**: Configuration files have explanatory comments

---

## 🤝 Contributing

Have an interesting stack to share? Contributions are welcome!

1. Fork the repository
2. Create your example following the existing structure
3. Add complete documentation
4. Submit a Pull Request

---

## 📚 Additional Documentation

- [Main Documentation](../README.md)
- [Quick Install Guide](../docs/QUICK-INSTALL.md)
- [CLI Commands Reference](../docs/CLI-COMMANDS.md)
- [CI/CD Pipeline](../docs/CI-CD-PIPELINE.md)

---

## 🆘 Support

If you have questions about the examples:

1. Read the specific example's README.md
2. Check the [main documentation](../README.md)
3. Open an issue on GitHub

---

**Made with ❤️ using go-overlay**
