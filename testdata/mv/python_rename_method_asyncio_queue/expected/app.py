import asyncio


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


async def use_get(qa: asyncio.Queue[A], qb: asyncio.Queue[B]):
    a = await qa.get()
    b = await qb.get()
    return a.execute() + b.run()


async def use_get_nowait(qa: asyncio.Queue[A], qb: asyncio.Queue[B]):
    a = qa.get_nowait()
    b = qb.get_nowait()
    return a.execute() + b.run()


async def use_inline(qa: asyncio.Queue[A], qb: asyncio.Queue[B]):
    return (await qa.get()).execute() + (await qb.get()).run()
