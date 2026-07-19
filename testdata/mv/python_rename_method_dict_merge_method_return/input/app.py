class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


class BoxA:
    def __init__(self):
        self.a = A()

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self):
        self.b = B()

    def get(self) -> B:
        return self.b


def use_merge_get_mr(ba: BoxA, bb: BoxB):
    return ({"k": ba.get()} | {}).get("k").run() + ({"k": bb.get()} | {}).get("k").run()


def use_merge_sub_mr(ba: BoxA, bb: BoxB):
    return ({"k": ba.get()} | {})["k"].run() + ({"k": bb.get()} | {})["k"].run()


def use_merge_assign_mr(ba: BoxA, bb: BoxB):
    da = {"k": ba.get()} | {}
    db = {"k": bb.get()} | {}
    return da["k"].run() + db["k"].run()


def use_merge_assign_get_mr(ba: BoxA, bb: BoxB):
    da = {} | {"k": ba.get()}
    db = {} | {"k": bb.get()}
    return da.get("k").run() + db.get("k").run()


def use_merge_rhs_mr(ba: BoxA, bb: BoxB):
    return ({} | {"k": ba.get()})["k"].run() + ({} | {"k": bb.get()})["k"].run()


# Class regression — already worked.
def use_merge_class():
    return ({"k": A()} | {}).get("k").run() + ({"k": B()} | {}).get("k").run()


def use_preserves_b(bb: BoxB):
    return ({"k": bb.get()} | {}).get("k").run() + ({} | {"k": bb.get()})["k"].run()
