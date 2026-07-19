from copy import copy, deepcopy
from collections import OrderedDict


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    a: A

    def __init__(self, a: A) -> None:
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    b: B

    def __init__(self, b: B) -> None:
        self.b = b

    def get(self) -> B:
        return self.b


# Class regressions — already solid.
def use_class_deepcopy_sub() -> int:
    return (
        deepcopy({"k": A()})["k"].execute()
        + deepcopy({"k": B()})["k"].run()
    )


def use_class_copy_sub() -> int:
    return (
        copy({"k": A()})["k"].execute()
        + copy({"k": B()})["k"].run()
    )


def use_class_deepcopy_get() -> int:
    return (
        deepcopy({"k": A()}).get("k").execute()
        + deepcopy({"k": B()}).get("k").run()
    )


def use_class_deepcopy_assign() -> int:
    da = deepcopy({"k": A()})
    db = deepcopy({"k": B()})
    return da["k"].execute() + db["k"].run()


def use_class_copy_assign() -> int:
    da = copy({"k": A()})
    db = copy({"k": B()})
    return da["k"].execute() + db["k"].run()


def use_class_deepcopy_od() -> int:
    return (
        deepcopy(OrderedDict(k=A()))["k"].execute()
        + deepcopy(OrderedDict(k=B()))["k"].run()
    )


def use_class_deepcopy_dict_ctor() -> int:
    return (
        deepcopy(dict(k=A()))["k"].execute()
        + deepcopy(dict(k=B()))["k"].run()
    )


# Method-return under foreign same-leaf.
def use_mr_deepcopy_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        deepcopy({"k": ba.get()})["k"].execute()
        + deepcopy({"k": bb.get()})["k"].run()
    )


def use_mr_copy_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        copy({"k": ba.get()})["k"].execute()
        + copy({"k": bb.get()})["k"].run()
    )


def use_mr_deepcopy_get(ba: BoxA, bb: BoxB) -> int:
    return (
        deepcopy({"k": ba.get()}).get("k").execute()
        + deepcopy({"k": bb.get()}).get("k").run()
    )


def use_mr_deepcopy_assign(ba: BoxA, bb: BoxB) -> int:
    da = deepcopy({"k": ba.get()})
    db = deepcopy({"k": bb.get()})
    return da["k"].execute() + db["k"].run()


def use_mr_copy_assign(ba: BoxA, bb: BoxB) -> int:
    da = copy({"k": ba.get()})
    db = copy({"k": bb.get()})
    return da["k"].execute() + db["k"].run()


def use_mr_deepcopy_od(ba: BoxA, bb: BoxB) -> int:
    return (
        deepcopy(OrderedDict(k=ba.get()))["k"].execute()
        + deepcopy(OrderedDict(k=bb.get()))["k"].run()
    )


def use_mr_deepcopy_dict_ctor(ba: BoxA, bb: BoxB) -> int:
    return (
        deepcopy(dict(k=ba.get()))["k"].execute()
        + deepcopy(dict(k=bb.get()))["k"].run()
    )


# Plain dict / list deepcopy regressions.
def use_mr_plain_dict(ba: BoxA, bb: BoxB) -> int:
    return {"k": ba.get()}["k"].execute() + {"k": bb.get()}["k"].run()


def use_mr_deepcopy_list(ba: BoxA, bb: BoxB) -> int:
    return deepcopy([ba.get()])[0].execute() + deepcopy([bb.get()])[0].run()
