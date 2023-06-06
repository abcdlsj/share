/**
 * Welcome to Cloudflare Workers! This is your first worker.
 *
 * - Run `npm run dev` in your terminal to start a development server
 * - Open a browser tab at http://localhost:8787/ to see your worker in action
 * - Run `npm run deploy` to publish your worker
 *
 * Learn more at https://developers.cloudflare.com/workers/
 */

const KEY = 'clip';

export interface Env {
	CLIP_KV: KVNamespace;
}

export default {
	async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
		if (request.method == 'POST') {
			const body = await request.text();
			await env.CLIP_KV.put(KEY, body);
			return new Response('ok');
		}

		const data = await env.CLIP_KV.get(KEY);
		return new Response(data);
	},
};
