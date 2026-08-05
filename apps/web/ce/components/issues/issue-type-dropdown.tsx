/**
 * Copyright (c) 2023-present Plane Software, Inc. and contributors
 * SPDX-License-Identifier: AGPL-3.0-only
 * See the LICENSE file for details.
 */

import { useRef, useState } from "react";
import { observer } from "mobx-react";
import { usePopper } from "react-popper";
import { Combobox } from "@headlessui/react";
// plane imports
import { useTranslation } from "@plane/i18n";
import { Logo } from "@plane/propel/emoji-icon-picker";
import { CheckIcon, ChevronDownIcon, SearchIcon } from "@plane/propel/icons";
import { ComboDropDown } from "@plane/ui";
import { cn } from "@plane/utils";
// components
import { DropdownButton } from "@/components/dropdowns/buttons";
import { BUTTON_VARIANTS_WITH_TEXT } from "@/components/dropdowns/constants";
import type { TDropdownProps } from "@/components/dropdowns/types";
// hooks
import { useDropdown } from "@/hooks/use-dropdown";
// plane web imports
import { useIssueTypes } from "@/plane-web/hooks/store";

export type TIssueTypeDropdownProps = TDropdownProps & {
  dropdownArrow?: boolean;
  dropdownArrowClassName?: string;
  onChange: (val: string) => void;
  onClose?: () => void;
  projectId: string | null | undefined;
  renderByDefault?: boolean;
  value: string | null | undefined;
};

export const IssueTypeDropdown = observer(function IssueTypeDropdown(props: TIssueTypeDropdownProps) {
  const {
    buttonClassName,
    buttonContainerClassName,
    buttonVariant,
    className = "",
    disabled = false,
    dropdownArrow = false,
    dropdownArrowClassName = "",
    hideIcon = false,
    onChange,
    onClose,
    placeholder,
    placement,
    projectId,
    renderByDefault = true,
    showTooltip = false,
    tabIndex,
    value,
  } = props;
  // refs
  const dropdownRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  // popper-js refs
  const [referenceElement, setReferenceElement] = useState<HTMLButtonElement | null>(null);
  const [popperElement, setPopperElement] = useState<HTMLDivElement | null>(null);
  // states
  const [query, setQuery] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  // plane hooks
  const { t } = useTranslation();
  // store hooks
  const { getIssueTypeById, getProjectIssueTypes } = useIssueTypes();
  // derived values
  const issueTypes = getProjectIssueTypes(projectId);
  const selectedIssueType = getIssueTypeById(value);
  const label = placeholder ?? t("issue.display.properties.issue_type");
  // popper-js init
  const { styles, attributes } = usePopper(referenceElement, popperElement, {
    placement: placement ?? "bottom-start",
    modifiers: [{ name: "preventOverflow", options: { padding: 12 } }],
  });
  // dropdown init
  const { handleClose, handleKeyDown, handleOnClick, searchInputKeyDown } = useDropdown({
    dropdownRef,
    inputRef,
    isOpen,
    onClose,
    query,
    setIsOpen,
    setQuery,
  });

  const filteredIssueTypes =
    query === ""
      ? issueTypes
      : issueTypes.filter((issueType) => issueType.name.toLowerCase().includes(query.toLowerCase()));

  const dropdownOnChange = (val: string) => {
    onChange(val);
    handleClose();
  };

  const comboButton = (
    <button
      tabIndex={tabIndex}
      ref={setReferenceElement}
      type="button"
      className={cn(
        "clickable block h-full max-w-full outline-none",
        {
          "cursor-not-allowed text-secondary": disabled,
          "cursor-pointer": !disabled,
        },
        buttonContainerClassName
      )}
      onClick={handleOnClick}
      disabled={disabled}
    >
      <DropdownButton
        className={buttonClassName}
        isActive={isOpen}
        tooltipHeading={label}
        tooltipContent={selectedIssueType?.name ?? label}
        showTooltip={showTooltip}
        variant={buttonVariant}
        renderToolTipByDefault={renderByDefault}
      >
        {!hideIcon && <Logo logo={selectedIssueType?.logo_props} size={14} type="lucide" />}
        {BUTTON_VARIANTS_WITH_TEXT.includes(buttonVariant) && (
          <span className="grow truncate text-left">{selectedIssueType?.name ?? label}</span>
        )}
        {dropdownArrow && (
          <ChevronDownIcon className={cn("h-2.5 w-2.5 shrink-0", dropdownArrowClassName)} aria-hidden="true" />
        )}
      </DropdownButton>
    </button>
  );

  return (
    // headlessui sets the combobox roles on the rendered elements, which the rule cannot see
    // eslint-disable-next-line jsx-a11y/no-static-element-interactions
    <ComboDropDown
      as="div"
      ref={dropdownRef}
      className={cn("h-full", className)}
      value={value}
      onChange={dropdownOnChange}
      disabled={disabled}
      onKeyDown={handleKeyDown}
      button={comboButton}
      renderByDefault={renderByDefault}
    >
      {isOpen && (
        <Combobox.Options className="fixed z-10" static>
          <div
            className="my-1 w-48 rounded-sm border-[0.5px] border-strong bg-surface-1 px-2 py-2.5 text-11 shadow-raised-200 focus:outline-none"
            ref={setPopperElement}
            style={styles.popper}
            {...attributes.popper}
          >
            <div className="flex items-center gap-1.5 rounded-sm border border-subtle bg-surface-2 px-2">
              <SearchIcon className="h-3.5 w-3.5 text-placeholder" strokeWidth={1.5} />
              <Combobox.Input
                as="input"
                ref={inputRef}
                className="w-full bg-transparent py-1 text-11 text-secondary placeholder:text-placeholder focus:outline-none"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t("common.search.label")}
                onKeyDown={searchInputKeyDown}
              />
            </div>
            <div className="mt-2 max-h-48 space-y-1 overflow-y-scroll">
              {filteredIssueTypes.length > 0 ? (
                filteredIssueTypes.map((issueType) => (
                  <Combobox.Option key={issueType.id} value={issueType.id}>
                    {({ active, selected }) => (
                      <div
                        className={cn(
                          "flex w-full cursor-pointer items-center justify-between gap-2 truncate rounded-sm px-1 py-1.5 select-none",
                          {
                            "bg-layer-transparent-hover": active,
                            "text-primary": selected,
                            "text-secondary": !selected,
                          }
                        )}
                      >
                        <span className="flex grow items-center gap-2 truncate">
                          <Logo logo={issueType.logo_props} size={14} type="lucide" />
                          <span className="grow truncate text-left">{issueType.name}</span>
                        </span>
                        {selected && <CheckIcon className="h-3.5 w-3.5 shrink-0" />}
                      </div>
                    )}
                  </Combobox.Option>
                ))
              ) : (
                <p className="px-1.5 py-1 text-placeholder italic">{t("no_matching_results")}</p>
              )}
            </div>
          </div>
        </Combobox.Options>
      )}
    </ComboDropDown>
  );
});
