class Box:
    def helper(self):
        return 1

    def stay(self):
        return 2


def use(obj):
    return obj.box.helper() + obj.box.stay()
