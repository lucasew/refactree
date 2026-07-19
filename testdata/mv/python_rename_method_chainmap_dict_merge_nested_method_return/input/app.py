from collections import ChainMap, OrderedDict


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    def __init__(self) -> None:
        self.a = A()

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self) -> None:
        self.b = B()

    def get(self) -> B:
        return self.b


# Class regression — already solid.
def use_class_chainmap_inline() -> int:
    return (
        ChainMap({"k": [A()]})["k"][0].run()
        + ChainMap({"k": [B()]})["k"][0].run()
    )


def use_class_chainmap_od() -> int:
    return (
        ChainMap(OrderedDict(k=[A()]))["k"][0].run()
        + ChainMap(OrderedDict(k=[B()]))["k"][0].run()
    )


def use_class_chainmap_multi() -> int:
    return (
        ChainMap({"k": [A()]}, {"m": [A()]})["k"][0].run()
        + ChainMap({"k": [B()]}, {"m": [B()]})["k"][0].run()
    )


def use_class_chainmap_get() -> int:
    return (
        ChainMap({"k": [A()]}).get("k")[0].run()
        + ChainMap({"k": [B()]}).get("k")[0].run()
    )


def use_class_chainmap_assign() -> int:
    ca = ChainMap({"k": [A()]})
    cb = ChainMap({"k": [B()]})
    return ca["k"][0].run() + cb["k"][0].run()


def use_class_dict_merge() -> int:
    return (
        ({"k": [A()]} | {"m": [A()]})["k"][0].run()
        + ({"k": [B()]} | {"m": [B()]})["k"][0].run()
    )


def use_class_dict_merge_get() -> int:
    return (
        ({"k": [A()]} | {}).get("k")[0].run()
        + ({"k": [B()]} | {}).get("k")[0].run()
    )


# Method-return under foreign same-leaf.
def use_mr_chainmap_inline(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": [ba.get()]})["k"][0].run()
        + ChainMap({"k": [bb.get()]})["k"][0].run()
    )


def use_mr_chainmap_od(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap(OrderedDict(k=[ba.get()]))["k"][0].run()
        + ChainMap(OrderedDict(k=[bb.get()]))["k"][0].run()
    )


def use_mr_chainmap_multi(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": [ba.get()]}, {"m": [ba.get()]})["k"][0].run()
        + ChainMap({"k": [bb.get()]}, {"m": [bb.get()]})["k"][0].run()
    )


def use_mr_chainmap_get(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": [ba.get()]}).get("k")[0].run()
        + ChainMap({"k": [bb.get()]}).get("k")[0].run()
    )


def use_mr_chainmap_assign(ba: BoxA, bb: BoxB) -> int:
    ca = ChainMap({"k": [ba.get()]})
    cb = ChainMap({"k": [bb.get()]})
    return ca["k"][0].run() + cb["k"][0].run()


def use_mr_dict_merge(ba: BoxA, bb: BoxB) -> int:
    return (
        ({"k": [ba.get()]} | {"m": [ba.get()]})["k"][0].run()
        + ({"k": [bb.get()]} | {"m": [bb.get()]})["k"][0].run()
    )


def use_mr_dict_merge_get(ba: BoxA, bb: BoxB) -> int:
    return (
        ({"k": [ba.get()]} | {}).get("k")[0].run()
        + ({"k": [bb.get()]} | {}).get("k")[0].run()
    )


# Single-level / scalar already solid regressions.
def use_mr_scalar_chainmap(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()})["k"].run()
        + ChainMap({"k": bb.get()})["k"].run()
    )


def use_mr_nested_dict(ba: BoxA, bb: BoxB) -> int:
    return (
        {"k": [ba.get()]}["k"][0].run()
        + {"k": [bb.get()]}["k"][0].run()
    )


def use_preserves_b(bb: BoxB) -> int:
    return (
        ChainMap({"k": [bb.get()]})["k"][0].run()
        + ChainMap(OrderedDict(k=[bb.get()]))["k"][0].run()
        + ChainMap({"k": [bb.get()]}).get("k")[0].run()
        + ({"k": [bb.get()]} | {"m": [bb.get()]})["k"][0].run()
        + ({"k": [bb.get()]} | {}).get("k")[0].run()
        + B().run()
    )
