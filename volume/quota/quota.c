#ifndef UNICODE
#define UNICODE
#endif

#ifndef CINTERFACE
#define CINTERFACE
#endif

#include <windows.h>
#include <stdio.h>
#include <fsrmquota.h>  // quota objects
#include "quota.h"

HRESULT __stdcall  SetQuota(WCHAR* volume, ULONGLONG limit) {
  HRESULT hr = S_OK;
  IFsrmQuotaManager* pqm = NULL;
  IFsrmQuota* pQuota = NULL;

  hr = initializeQuotaManager(&pqm);
  if (FAILED(hr)) {
    cleanup(pqm, pQuota);
    return hr;
  }

  BSTR v = SysAllocString(volume);
  hr = pqm->lpVtbl->CreateQuota(pqm, v, &pQuota);
  SysFreeString(v);

  if (FAILED(hr))
  {
    wprintf(L"pqm->CreateQuota failed, 0x%x.\n", hr);
    cleanup(pqm, pQuota);
    return hr;
  }

  VARIANT l;
  l.vt = VT_UI8;
  l.ullVal = limit;
  hr = pQuota->lpVtbl->put_QuotaLimit(pQuota, l);
  if (FAILED(hr))
  {
    wprintf(L"pQuota->put_QuotaLimit failed, 0x%x.\n", hr);
    cleanup(pqm, pQuota);
    return hr;
  }

  hr = pQuota->lpVtbl->Commit(pQuota);
  if (FAILED(hr))
  {
    wprintf(L"pQuota->Commit failed, 0x%x.\n", hr);
    cleanup(pqm, pQuota);
    return hr;
  }

  cleanup(pqm, pQuota);
  return S_OK;
}


HRESULT __stdcall  GetQuotaUsed(WCHAR* volume, PULONGLONG quotaUsed) {
  HRESULT hr = S_OK;
  IFsrmQuotaManager* pqm = NULL;
  IFsrmQuota* pQuota = NULL;

  hr = initializeQuotaManager(&pqm);
  if (FAILED(hr)) {
    cleanup(pqm, pQuota);
    return hr;
  }

  BSTR v = SysAllocString(volume);
  hr = pqm->lpVtbl->GetQuota(pqm, v, &pQuota);
  SysFreeString(v);

  if (hr == FSRM_E_NOT_FOUND)
  {
    *quotaUsed = 0;
    cleanup(pqm, pQuota);
    return S_OK;
  } else if (FAILED(hr))
  {
    wprintf(L"pqm->GetQuota failed, 0x%x.\n", hr);
    cleanup(pqm, pQuota);
    return hr;
  }


  VARIANT l;
  VariantInit(&l);
  hr = pQuota->lpVtbl->get_QuotaUsed(pQuota, &l);
  if (FAILED(hr))
  {
    wprintf(L"pQuota->get_QuotaUsed failed, 0x%x.\n", hr);
    cleanup(pqm, pQuota);
    return hr;
  }

  *quotaUsed = l.ullVal;
  cleanup(pqm, pQuota);
  return S_OK;
}

HRESULT initializeQuotaManager(IFsrmQuotaManager** pqmOut) {
  HRESULT hr = S_OK;
  IID CLSID_FsrmQuotaManager;
  IID IID_IFsrmQuotaManagerEx;

  hr = IIDFromString(OLESTR("{90dcab7f-347c-4bfc-b543-540326305fbe}"), &CLSID_FsrmQuotaManager);
  if (FAILED(hr))
  {
    wprintf(L"IIDFromString(CLSID_FsrmQuotaManager) failed, 0x%x.\n", hr);
    return hr;
  }

  hr = IIDFromString(OLESTR("{4846cb01-d430-494f-abb4-b1054999fb09}"), &IID_IFsrmQuotaManagerEx);
  if (FAILED(hr))
  {
    wprintf(L"IIDFromString(IID_FsrmQuotaManager) failed, 0x%x.\n", hr);
    return hr;
  }

  hr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
  if (FAILED(hr))
  {
    wprintf(L"CoInitializeEx() failed, 0x%x.\n", hr);
    return hr;
  }

  hr = CoCreateInstance(&CLSID_FsrmQuotaManager,
    NULL,
    CLSCTX_LOCAL_SERVER,
    &IID_IFsrmQuotaManagerEx,
    (void**)(pqmOut));

  if (FAILED(hr))
  {
    wprintf(L"CoCreateInstance(FsrmQuotaManager) failed, 0x%x.\n", hr);
    return hr;
  }

  return S_OK;
}
void cleanup(IFsrmQuotaManager* pqm, IFsrmQuota* pQuota) {
  if (pqm)
    pqm->lpVtbl->Release(pqm);

  if (pQuota)
    pQuota->lpVtbl->Release(pQuota);

  CoUninitialize();
}

