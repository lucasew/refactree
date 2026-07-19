import copy
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


# Class regressions — already solid / product module-qualified.
def use_class_deepcopy_sub() -> int:
    return (
        copy.deepcopy({"k": A()})["k"].execute()
        + copy.deepcopy({"k": B()})["k"].run()
    )


def use_class_copy_sub() -> int:
    return (
        copy.copy({"k": A()})["k"].execute()
        + copy.copy({"k": B()})["k"].run()
    )


def use_class_deepcopy_get() -> int:
    return (
        copy.deepcopy({"k": A()}).get("k").execute()
        + copy.deepcopy({"k": B()}).get("k").run()
    )


def use_class_copy_get() -> int:
    return (
        copy.copy({"k": A()}).get("k").execute()
        + copy.copy({"k": B()}).get("k").run()
    )


def use_class_deepcopy_assign() -> int:
    da = copy.deepcopy({"k": A()})
    db = copy.deepcopy({"k": B()})
    return da["k"].execute() + db["k"].run()


def use_class_copy_assign() -> int:
    da = copy.copy({"k": A()})
    db = copy.copy({"k": B()})
    return da["k"].execute() + db["k"].run()


def use_class_deepcopy_values() -> int:
    return (
        list(copy.deepcopy({"k": A()}).values())[0].execute()
        + list(copy.deepcopy({"k": B()}).values())[0].run()
    )


def use_class_deepcopy_od() -> int:
    return (
        copy.deepcopy(OrderedDict(k=A()))["k"].execute()
        + copy.deepcopy(OrderedDict(k=B()))["k"].run()
    )


def use_class_deepcopy_dict_ctor() -> int:
    return (
        copy.deepcopy(dict(k=A()))["k"].execute()
        + copy.deepcopy(dict(k=B()))["k"].run()
    )


# Method-return under foreign same-leaf.
def use_mr_deepcopy_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        copy.deepcopy({"k": ba.get()})["k"].execute()
        + copy.deepcopy({"k": bb.get()})["k"].run()
    )


def use_mr_copy_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        copy.copy({"k": ba.get()})["k"].execute()
        + copy.copy({"k": bb.get()})["k"].run()
    )


def use_mr_deepcopy_get(ba: BoxA, bb: BoxB) -> int:
    return (
        copy.deepcopy({"k": ba.get()}).get("k").execute()
        + copy.deepcopy({"k": bb.get()}).get("k").run()
    )


def use_mr_copy_get(ba: BoxA, bb: BoxB) -> int:
    return (
        copy.copy({"k": ba.get()}).get("k").execute()
        + copy.copy({"k": bb.get()}).get("k").run()
    )


def use_mr_deepcopy_assign(ba: BoxA, bb: BoxB) -> int:
    da = copy.deepcopy({"k": ba.get()})
    db = copy.deepcopy({"k": bb.get()})
    return da["k"].execute() + db["k"].run()


def use_mr_copy_assign(ba: BoxA, bb: BoxB) -> int:
    da = copy.copy({"k": ba.get()})
    db = copy.copy({"k": bb.get()})
    return da["k"].execute() + db["k"].run()


def use_mr_deepcopy_values(ba: BoxA, bb: BoxB) -> int:
    return (
        list(copy.deepcopy({"k": ba.get()}).values())[0].execute()
        + list(copy.deepcopy({"k": bb.get()}).values())[0].run()
    )


def use_mr_deepcopy_od(ba: BoxA, bb: BoxB) -> int:
    return (
        copy.deepcopy(OrderedDict(k=ba.get()))["k"].execute()
        + copy.deepcopy(OrderedDict(k=bb.get()))["k"].run()
    )


def use_mr_deepcopy_dict_ctor(ba: BoxA, bb: BoxB) -> int:
    return (
        copy.deepcopy(dict(k=ba.get()))["k"].execute()
        + copy.deepcopy(dict(k=bb.get()))["k"].run()
    )


# Bare from-import / list / plain dict regressions.
def use_mr_bare_deepcopy(ba: BoxA, bb: BoxB) -> int:
    from copy import deepcopy

    return deepcopy({"k": ba.get()})["k"].execute() + deepcopy({"k": bb.get()})["k"].run()


def use_mr_deepcopy_list(ba: BoxA, bb: BoxB) -> int:
    return copy.deepcopy([ba.get()])[0].execute() + copy.deepcopy([bb.get()])[0].run()


def use_mr_plain_dict(ba: BoxA, bb: BoxB) -> int:
    return {"k": ba.get()}["k"].execute() + {"k": bb.get()}["k"].run()


# Preserves B.
def use_preserves_b(bb: BoxB) -> int:
    return copy.deepcopy({"k": bb.get()})["k"].run()
