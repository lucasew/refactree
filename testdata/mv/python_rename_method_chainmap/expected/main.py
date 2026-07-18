from collections import ChainMap


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_chainmap_sub():
    ca: ChainMap[str, A] = ChainMap({"k": A()})
    cb: ChainMap[str, B] = ChainMap({"k": B()})
    return ca["k"].execute() + cb["k"].run()


def use_chainmap_get():
    ca: ChainMap[str, A] = ChainMap({"k": A()})
    cb: ChainMap[str, B] = ChainMap({"k": B()})
    return ca.get("k").execute() + cb.get("k").run()


def use_chainmap_values():
    ca: ChainMap[str, A] = ChainMap({"k": A()})
    cb: ChainMap[str, B] = ChainMap({"k": B()})
    n = 0
    for a in ca.values():
        n += a.execute()
    for b in cb.values():
        n += b.run()
    return n


def use_chainmap_items():
    ca: ChainMap[str, A] = ChainMap({"k": A()})
    cb: ChainMap[str, B] = ChainMap({"k": B()})
    n = 0
    for k, a in ca.items():
        n += a.execute()
    for k, b in cb.items():
        n += b.run()
    return n


def use_preserves_b():
    cb: ChainMap[str, B] = ChainMap({"k": B()})
    return cb["k"].run() + cb.get("k").run()
