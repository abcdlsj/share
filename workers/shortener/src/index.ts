/**
 * Welcome to Cloudflare Workers! This is your first worker.
 *
 * - Run `wrangler dev src/index.ts` in your terminal to start a development server
 * - Open a browser tab at http://localhost:8787/ to see your worker in action
 * - Run `wrangler publish src/index.ts --name my-worker` to publish your worker
 *
 * Learn more at https://developers.cloudflare.com/workers/
 */

import { Router } from "itty-router";
const router = Router();

export interface Env {
	SHORTENER_KV: KVNamespace;
	HOST_URL: string;
}

router.get("/:shr", async (req: Request, env: Env) => {
	const shr = req.params.shr;
	let longLink = await env.SHORTENER_KV.get(shr);
	if (!longLink) {
		return new Response("Not Found", { status: 404 });
	}
	return Response.redirect(longLink, 302);
});

router.post("/", async (req: Request, env: Env) => {
	const json = await req.json();
	const longLink = json?.url;
	if (!longLink) {
		return new Response("Bad Request", { status: 400 });
	}
	let shr = Math.random().toString(32).slice(-7);
	let existing = await env.SHORTENER_KV.get(shr);
	while (existing) {
		shr = Math.random().toString(36).slice(-7);
		existing = await env.SHORTENER_KV.get(shr);
	}
	await env.SHORTENER_KV.put(shr, longLink);
	return new Response(JSON.stringify({ url: `${env.HOST_URL}/${shr}` }), {
		headers: { "content-type": "application/json" },
	});
});

router.get("/", async (req: Request, env: Env) => {
	let list = await env.SHORTENER_KV.list()
	let urls = new Map()
	for await (const key of list.keys) {
		urls.set(`${env.HOST_URL}/${key.name}`, await env.SHORTENER_KV.get(key.name))
	}
	return new Response(JSON.stringify(Object.fromEntries(urls)), {
		headers: { "content-type": "application/json" },
	});
});


router.post("/c", async (req: Request, env: Env) => {
	let list = await env.SHORTENER_KV.list()
	for await (const key of list.keys) {
		await env.SHORTENER_KV.delete(key.name)
	}
	return new Response("Cleared", {
		headers: { "content-type": "application/json" },
	});
});

export default {
	async fetch(
		request: Request,
		env: Env,
		ctx: ExecutionContext
	): Promise<Response> {
		return router.handle(request, env, ctx).then((res: Response) => {
			return res;
		});
	},
};
