<script lang="ts">
	import { onMount } from "svelte";
	import { connect } from "../sources/actor";
	import type { Readable } from "svelte/store";

	import { defaultStage } from "../constants/endpoints";
	export let stage = defaultStage;
	export let actorID = "actor";
	let lastStageMessage : Readable<any>;

	onMount(() => {
		lastStageMessage = connect(stage, actorID);
	});
</script>

<main>
	<pre>
		{JSON.stringify($lastStageMessage)}
	</pre>
</main>
