class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_walrus_list():
    if (aa := [A()]):
        xa = aa[0].execute()
    if (bb := [B()]):
        xb = bb[0].run()
    return xa + xb


def use_walrus_dict():
    if (da := {"k": A()}):
        xa = da["k"].execute()
    if (db := {"k": B()}):
        xb = db["k"].run()
    return xa + xb


def use_walrus_nested():
    if (aa := [[A()]]):
        xa = aa[0][0].execute()
    if (bb := [[B()]]):
        xb = bb[0][0].run()
    return xa + xb


def use_nested_row_mid():
    aa = [[A()]][0]
    bb = [[B()]][0]
    return aa[0].execute() + bb[0].run()
