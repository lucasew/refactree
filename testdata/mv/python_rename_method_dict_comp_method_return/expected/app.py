class A:
    def execute(self):
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


# Dict comprehension method-return under foreign same-leaf.
def use_dict_comp_mr(ba: BoxA, bb: BoxB):
    da_cm = {k: ba.get() for k in ["k"]}
    db_cm = {k: bb.get() for k in ["k"]}
    return da_cm["k"].execute() + db_cm["k"].run()


def use_dict_comp_mr_assign(ba: BoxA, bb: BoxB):
    da_cma = {k: ba.get() for k in ["k"]}
    db_cma = {k: bb.get() for k in ["k"]}
    xa = da_cma["k"]
    xb = db_cma["k"]
    return xa.execute() + xb.run()


def use_dict_comp_mr_values(ba: BoxA, bb: BoxB):
    da_cmv = {k: ba.get() for k in ["k"]}
    db_cmv = {k: bb.get() for k in ["k"]}
    n = 0
    for a in da_cmv.values():
        n += a.execute()
    for b in db_cmv.values():
        n += b.run()
    return n


def use_dict_comp_mr_multi(ba: BoxA, bb: BoxB):
    da_cmm = {k: ba.get() for k in ["k", "m"]}
    db_cmm = {k: bb.get() for k in ["k", "m"]}
    return da_cmm["k"].execute() + da_cmm["m"].execute() + db_cmm["k"].run() + db_cmm["m"].run()


# Class regression — already worked.
def use_dict_comp_class():
    da_cc = {k: A() for k in ["k"]}
    db_cc = {k: B() for k in ["k"]}
    return da_cc["k"].execute() + db_cc["k"].run()


def use_preserves_b(bb: BoxB):
    db_pb = {k: bb.get() for k in ["k"]}
    return db_pb["k"].run()
